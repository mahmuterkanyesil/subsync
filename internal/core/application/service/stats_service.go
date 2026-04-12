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
	watchDirRepo port.WatchDirRepository
	taskQueue    port.TaskQueue
}

func NewStatsService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	watchDirRepo port.WatchDirRepository,
	taskQueue port.TaskQueue,
) *StatsService {
	return &StatsService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		watchDirRepo: watchDirRepo,
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
	return s.taskQueue.Enqueue(ctx, "translate_srt", port.TranslateTask{
		EngPath: engPath,
	})
}

func (s *StatsService) ReEmbed(ctx context.Context, engPath string) error {
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}
	// embedded → done veya embed_failed → done geçişi yaparak embedder'ın tekrar almasını sağla
	status := subtitle.Status()
	if status == valueobject.StatusEmbedded || status == valueobject.StatusEmbedFailed {
		if err := subtitle.TransitionTo(valueobject.StatusDone); err != nil {
			return err
		}
		subtitle.MarkUnembedded()
	}
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

func (s *StatsService) ListAPIKeys(ctx context.Context) ([]*entity.APIKey, error) {
	return s.apiKeyRepo.FindAll(ctx)
}

func (s *StatsService) DeleteAPIKey(ctx context.Context, id int) error {
	return s.apiKeyRepo.Delete(ctx, id)
}

func (s *StatsService) ActivateAPIKey(ctx context.Context, id int) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	key.Activate()
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) ListRecordsByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error) {
	return s.subtitleRepo.FindByStatus(ctx, status)
}

func (s *StatsService) ListWatchDirs(ctx context.Context) ([]*entity.WatchDir, error) {
	return s.watchDirRepo.FindAll(ctx)
}

func (s *StatsService) AddWatchDir(ctx context.Context, path string) error {
	wd, err := entity.NewWatchDir(path)
	if err != nil {
		return err
	}
	return s.watchDirRepo.Save(ctx, wd)
}

func (s *StatsService) DeleteWatchDir(ctx context.Context, id int) error {
	return s.watchDirRepo.Delete(ctx, id)
}

func (s *StatsService) ToggleWatchDir(ctx context.Context, id int) error {
	wd, err := s.watchDirRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	wd.Toggle()
	return s.watchDirRepo.Save(ctx, wd)
}
