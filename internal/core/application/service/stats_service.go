package service

import (
	"context"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
)

type StatsService struct {
	subtitleRepo port.SubtitleRepository
	apiKeyRepo   port.APIKeyRepository
	taskQueue    port.TaskQueue
}

func NewStatsService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	taskQueue port.TaskQueue,
) *StatsService {
	return &StatsService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		taskQueue:    taskQueue,
	}
}

func (s *StatsService) GetStats(ctx context.Context) (port.SubtitleStats, error) {
	return s.subtitleRepo.Statistics(ctx)
}

func (s *StatsService) ListRecords(ctx context.Context) ([]*entity.Subtitle, error) {
	return s.subtitleRepo.FindAll(ctx)
}

func (s *StatsService) FindByPath(ctx context.Context, engPath string) (*entity.Subtitle, error) {
	return s.subtitleRepo.FindByPath(ctx, engPath)
}

func (s *StatsService) ReTranslate(ctx context.Context, engPath string) error {
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}
	if err := subtitle.TransitionTo(valueobject.StatusQueued); err != nil {
		return err
	}
	if err := s.subtitleRepo.Save(ctx, subtitle); err != nil {
		return err
	}
	return s.taskQueue.Enqueue(ctx, "translate_srt", map[string]string{
		"eng_path": engPath,
	})
}

func (s *StatsService) ReEmbed(ctx context.Context, engPath string) error {
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}
	subtitle.MarkEmbedded()
	return s.subtitleRepo.Save(ctx, subtitle)
}

func (s *StatsService) AddApiKey(ctx context.Context, service string, keyValue string) error {
	key, err := entity.NewAPIKey(service, keyValue)
	if err != nil {
		return err
	}
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) DisableApiKey(ctx context.Context, id int) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	key.Deactivate()
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) ResetQuotaApiKey(ctx context.Context, id int) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	key.ResetQuota()
	return s.apiKeyRepo.Save(ctx, key)
}
