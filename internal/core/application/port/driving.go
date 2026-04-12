package port

import (
	"context"
	"subsync/internal/core/domain/entity"
)

type ScanningUseCase interface {
	Scan(ctx context.Context) error
}

type TranslationUseCase interface {
	Translate(ctx context.Context, engPath string) error
}

type EmbeddingUseCase interface {
	EmbedPending(ctx context.Context) error
}

type StatsUseCase interface {
	GetStats(ctx context.Context) (SubtitleStats, error)
	ListRecords(ctx context.Context) ([]*entity.Subtitle, error)
	FindByPath(ctx context.Context, engPath string) (*entity.Subtitle, error)
	ReTranslate(ctx context.Context, engPath string) error
	ReEmbed(ctx context.Context, engPath string) error
	AddApiKey(ctx context.Context, service string, keyValue string) error
	DisableApiKey(ctx context.Context, id int) error
	ResetQuotaApiKey(ctx context.Context, id int) error
}
