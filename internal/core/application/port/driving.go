package port

import (
	"context"
	"subsync/internal/core/domain/entity"
	valueobject "subsync/internal/core/domain/valueobject"
)

type ScanningUseCase interface {
	Scan(ctx context.Context) error
}

type TranslationUseCase interface {
	Translate(ctx context.Context, engPath, targetLang string) error
}

type EmbeddingUseCase interface {
	EmbedPending(ctx context.Context) error
}

type StatsUseCase interface {
	GetStats(ctx context.Context) (*SubtitleStats, error)
	ListRecords(ctx context.Context) ([]*entity.Subtitle, error)
	FindByPath(ctx context.Context, engPath string) (*entity.Subtitle, error)
	ReTranslate(ctx context.Context, engPath string) error
	ReEmbed(ctx context.Context, engPath string) error
	AddApiKey(ctx context.Context, service, keyValue, model string) error
	UpdateApiKeyModel(ctx context.Context, id int, model string) error
	DisableApiKey(ctx context.Context, id int) error
	ResetQuotaApiKey(ctx context.Context, id int) error
	ListAPIKeys(ctx context.Context) ([]*entity.APIKey, error)
	ListAPIKeysWithUsage(ctx context.Context) ([]APIKeyWithUsage, error)
	DeleteAPIKey(ctx context.Context, id int) error
	ActivateAPIKey(ctx context.Context, id int) error
	ListRecordsByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error)
	DeleteSubtitle(ctx context.Context, engPath string) error
	RefreshKeyStatuses(ctx context.Context) error
	GetTargetLanguage(ctx context.Context) string
	SetTargetLanguage(ctx context.Context, code string) error
	ListWatchDirs(ctx context.Context) ([]*entity.WatchDir, error)
	AddWatchDir(ctx context.Context, path string) error
	DeleteWatchDir(ctx context.Context, id int) error
	ToggleWatchDir(ctx context.Context, id int) error
}
