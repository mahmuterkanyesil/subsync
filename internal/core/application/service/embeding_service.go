package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/event"
	"subsync/internal/core/domain/valueobject"
	"sync"
)

type EmbeddingService struct {
	subtitleRepo   port.SubtitleRepository
	videoProcessor port.VideoProcessor
	events         port.EventPublisher
	mu             sync.Mutex
	inProgress     map[string]struct{}
}

func NewEmbeddingService(
	subtitleRepo port.SubtitleRepository,
	videoProcessor port.VideoProcessor,
	events port.EventPublisher,
) *EmbeddingService {
	return &EmbeddingService{
		subtitleRepo:   subtitleRepo,
		videoProcessor: videoProcessor,
		events:         events,
		inProgress:     make(map[string]struct{}),
	}
}

func findVideoPath(engPath string) (string, error) {
	base := strings.TrimSuffix(engPath, ".eng.srt")
	for _, ext := range []string{".mkv", ".mp4"} {
		if _, err := os.Stat(base + ext); err == nil {
			return base + ext, nil
		}
	}
	return "", port.ErrVideoNotFound
}

func (s *EmbeddingService) publish(e event.DomainEvent) {
	if s.events != nil {
		s.events.Publish(e)
	}
}

func (s *EmbeddingService) EmbedPending(ctx context.Context) error {
	pending, err := s.subtitleRepo.FindPendingEmbed(ctx)
	if err != nil {
		return err
	}

	for _, subtitle := range pending {
		s.mu.Lock()
		if _, busy := s.inProgress[subtitle.EngPath()]; busy {
			s.mu.Unlock()
			continue
		}
		s.inProgress[subtitle.EngPath()] = struct{}{}
		s.mu.Unlock()

		s.embedOne(ctx, subtitle)

		s.mu.Lock()
		delete(s.inProgress, subtitle.EngPath())
		s.mu.Unlock()
	}

	return nil
}

func (s *EmbeddingService) embedOne(ctx context.Context, subtitle *entity.Subtitle) {
	videoPath, err := findVideoPath(subtitle.EngPath())
	if err != nil {
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(port.ErrVideoNotFound)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingFailed(subtitle.EngPath(), port.ErrVideoNotFound.Error()))
		return
	}

	// Already embedded?
	hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, videoPath)
	if err == nil && hasTr {
		subtitle.MarkEmbedded()
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingCompleted(subtitle.EngPath(), videoPath))
		return
	}

	// eng.srt > 2MB = corrupt
	if info, statErr := os.Stat(subtitle.EngPath()); statErr == nil && info.Size() > 2*1024*1024 {
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(port.ErrEngSrtTooLarge)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingFailed(subtitle.EngPath(), port.ErrEngSrtTooLarge.Error()))
		return
	}

	engPath := subtitle.EngPath()
	trPath := engPath[:len(engPath)-len(".eng.srt")] + ".tr.srt"

	if embedErr := s.videoProcessor.EmbedSubtitle(ctx, videoPath, trPath); embedErr != nil {
		s.handleEmbedError(ctx, subtitle, videoPath, embedErr)
		return
	}

	subtitle.MarkEmbedded()
	_ = s.subtitleRepo.Save(ctx, subtitle)
	s.publish(event.NewEmbeddingCompleted(subtitle.EngPath(), videoPath))
}

func (s *EmbeddingService) handleEmbedError(ctx context.Context, subtitle *entity.Subtitle, videoPath string, err error) {
	switch {
	case errors.Is(err, port.ErrVideoNotFound),
		errors.Is(err, port.ErrFFmpegNotFound):
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(err)
		s.publish(event.NewEmbeddingFailed(subtitle.EngPath(), err.Error()))
	case errors.Is(err, port.ErrTrSrtNotFound):
		_ = subtitle.TransitionTo(valueobject.StatusQueued)
	case errors.Is(err, port.ErrFFmpegFailed),
		errors.Is(err, port.ErrOutputTooSmall):
		// transient — stay in StatusDone, retry next cycle
		subtitle.MarkError(err)
	default:
		subtitle.MarkError(err)
	}
	_ = s.subtitleRepo.Save(ctx, subtitle)
}
