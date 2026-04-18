package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/event"
	domainservice "subsync/internal/core/domain/service"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/logger"
	"subsync/pkg/srt"
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
	name := filepath.Base(subtitle.EngPath())

	videoPath, err := findVideoPath(subtitle.EngPath())
	if err != nil {
		logger.Warn("embed: video not found for %s", name)
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(port.ErrVideoNotFound)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingFailed(subtitle.EngPath(), port.ErrVideoNotFound.Error()))
		return
	}

	// Already embedded?
	hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, videoPath)
	if err == nil && hasTr {
		logger.Info("embed: already has TR sub, marking embedded: %s", name)
		subtitle.MarkEmbedded()
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingCompleted(subtitle.EngPath(), videoPath))
		return
	}

	// eng.srt > 2MB = corrupt
	if info, statErr := os.Stat(subtitle.EngPath()); statErr == nil && info.Size() > 2*1024*1024 {
		logger.Warn("embed: eng.srt too large (%dMB): %s", info.Size()/1024/1024, name)
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(port.ErrEngSrtTooLarge)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingFailed(subtitle.EngPath(), port.ErrEngSrtTooLarge.Error()))
		return
	}

	engPath := subtitle.EngPath()
	trPath := engPath[:len(engPath)-len(".eng.srt")] + ".tr.srt"

	if anomalyErr := s.checkSubtitleAnomaly(engPath, trPath); anomalyErr != nil {
		logger.Warn("embed: anomaly check failed for %s — %v", name, anomalyErr)
		if errors.Is(anomalyErr, os.ErrNotExist) {
			_ = subtitle.TransitionTo(valueobject.StatusQueued)
		} else {
			_ = os.Remove(trPath)
			_ = subtitle.TransitionTo(valueobject.StatusError)
			subtitle.MarkError(fmt.Errorf("anomaly: %w", anomalyErr))
		}
		_ = s.subtitleRepo.Save(ctx, subtitle)
		s.publish(event.NewEmbeddingFailed(subtitle.EngPath(), anomalyErr.Error()))
		return
	}

	logger.Info("embed start: %s → %s", name, filepath.Base(videoPath))
	if embedErr := s.videoProcessor.EmbedSubtitle(ctx, videoPath, trPath); embedErr != nil {
		logger.Error("embed failed: %s — %v", name, embedErr)
		s.handleEmbedError(ctx, subtitle, videoPath, embedErr)
		return
	}

	logger.Info("embed done: %s", name)
	subtitle.MarkEmbedded()
	_ = s.subtitleRepo.Save(ctx, subtitle)
	s.publish(event.NewEmbeddingCompleted(subtitle.EngPath(), videoPath))
}

func (s *EmbeddingService) checkSubtitleAnomaly(engPath, trPath string) error {
	engContent, err := os.ReadFile(engPath)
	if err != nil {
		return fmt.Errorf("cannot read eng.srt: %w", err)
	}
	trContent, err := os.ReadFile(trPath)
	if err != nil {
		return fmt.Errorf("tr.srt not found: %w", err)
	}
	if len(strings.TrimSpace(string(trContent))) == 0 {
		return fmt.Errorf("tr.srt is empty")
	}
	engBlocks := srt.Parse(string(engContent))
	trBlocks := srt.Parse(string(trContent))
	return domainservice.ValidateTranslation(engBlocks, trBlocks)
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
