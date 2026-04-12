package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
	"subsync/internal/infrastructure/adapter/video/ffmpeg"
	"sync"
)

type EmbeddingService struct {
	subtitleRepo   port.SubtitleRepository
	videoProcessor port.VideoProcessor
	mu             sync.Mutex
	inProgress     map[string]struct{}
}

func NewEmbeddingService(
	subtitleRepo port.SubtitleRepository,
	videoProcessor port.VideoProcessor,
) *EmbeddingService {
	return &EmbeddingService{
		subtitleRepo:   subtitleRepo,
		videoProcessor: videoProcessor,
		inProgress:     make(map[string]struct{}),
	}
}

func findVideoPath(engPath string) (string, error) {
	base := strings.TrimSuffix(engPath, ".eng.srt")
	for _, ext := range []string{".mkv", ".mp4"} {
		candidate := base + ext
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", ffmpeg.ErrVideoNotFound
}

func (s *EmbeddingService) EmbedPending(ctx context.Context) error {
	pending, err := s.subtitleRepo.FindPendingEmbed(ctx)
	if err != nil {
		return err
	}

	for _, subtitle := range pending {
		// File lock: skip if already in progress
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
	// Find actual video file
	videoPath, err := findVideoPath(subtitle.EngPath())
	if err != nil {
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(ffmpeg.ErrVideoNotFound)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return
	}

	// Turkish subtitle already embedded?
	hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, videoPath)
	if err == nil && hasTr {
		subtitle.MarkEmbedded()
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return
	}

	// eng.srt size check: > 2MB is corrupt
	if info, statErr := os.Stat(subtitle.EngPath()); statErr == nil && info.Size() > 2*1024*1024 {
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(ffmpeg.ErrEngSrtTooLarge)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return
	}

	// Build .tr.srt path
	engPath := subtitle.EngPath()
	trPath := engPath[:len(engPath)-len(".eng.srt")] + ".tr.srt"

	// Embed
	if embedErr := s.videoProcessor.EmbedSubtitle(ctx, videoPath, trPath); embedErr != nil {
		s.handleEmbedError(ctx, subtitle, embedErr)
		return
	}

	subtitle.MarkEmbedded()
	_ = s.subtitleRepo.Save(ctx, subtitle)
}

func (s *EmbeddingService) handleEmbedError(ctx context.Context, subtitle *entity.Subtitle, err error) {
	switch {
	case errors.Is(err, ffmpeg.ErrVideoNotFound),
		errors.Is(err, ffmpeg.ErrFFmpegNotFound):
		_ = subtitle.TransitionTo(valueobject.StatusEmbedFailed)
		subtitle.MarkError(err)
	case errors.Is(err, ffmpeg.ErrTrSrtNotFound):
		_ = subtitle.TransitionTo(valueobject.StatusQueued)
	case errors.Is(err, ffmpeg.ErrFFmpegFailed),
		errors.Is(err, ffmpeg.ErrOutputTooSmall):
		// transient — stay in StatusDone, retry next cycle
		subtitle.MarkError(err)
	default:
		subtitle.MarkError(err)
	}
	_ = s.subtitleRepo.Save(ctx, subtitle)
}
