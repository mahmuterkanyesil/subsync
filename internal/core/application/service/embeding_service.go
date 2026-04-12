package service

import (
	"context"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/valueobject"
)

type EmbeddingService struct {
	subtitleRepo   port.SubtitleRepository
	videoProcessor port.VideoProcessor
}

func NewEmbeddingService(
	subtitleRepo port.SubtitleRepository,
	videoProcessor port.VideoProcessor,
) *EmbeddingService {
	return &EmbeddingService{
		subtitleRepo:   subtitleRepo,
		videoProcessor: videoProcessor,
	}
}

func (s *EmbeddingService) EmbedPending(ctx context.Context) error {
	// 1. Bekleyen embed'leri getir
	pending, err := s.subtitleRepo.FindPendingEmbed(ctx)
	if err != nil {
		return err
	}

	for _, subtitle := range pending {
		// 2. Türkçe subtitle zaten var mı?
		hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, subtitle.EngPath())
		if err != nil {
			continue
		}
		if hasTr {
			subtitle.MarkEmbedded(true)
			_ = s.subtitleRepo.Save(ctx, subtitle)
			continue
		}

		// 3. .tr.srt path'i oluştur
		trPath := subtitle.EngPath()
		trPath = trPath[:len(trPath)-len(".eng.srt")] + ".tr.srt"

		// 4. Embed et
		err = s.videoProcessor.EmbedSubtitle(ctx, subtitle.EngPath(), trPath)
		if err != nil {
			_ = subtitle.TransitionTo(valueobject.StatusError)
			_ = s.subtitleRepo.Save(ctx, subtitle)
			continue
		}

		// 5. Embedded olarak işaretle
		subtitle.MarkEmbedded(true)
		_ = s.subtitleRepo.Save(ctx, subtitle)
	}

	return nil
}
