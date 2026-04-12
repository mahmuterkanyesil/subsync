package port

import (
	"context"
	"subsync/internal/core/domain/entity"
)

type SubtitleStats struct {
	Total          int
	Done           int
	Queued         int
	Error          int
	QuotaExhausted int
	Embedded       int
}

type SubtitleRepository interface {
	Save(ctx context.Context, s *entity.Subtitle) error
	FindByPath(ctx context.Context, path string) (*entity.Subtitle, error)
	FindAll(ctx context.Context) ([]*entity.Subtitle, error)
	FindPendingEmbed(ctx context.Context) ([]*entity.Subtitle, error)
	Statistics(ctx context.Context) (SubtitleStats, error)
}

type APIKeyRepository interface {
	Save(ctx context.Context, k *entity.APIKey) error
	FindByID(ctx context.Context, id int) (*entity.APIKey, error)
	FindAll(ctx context.Context) ([]*entity.APIKey, error)
	FindByService(ctx context.Context, service string) ([]*entity.APIKey, error)
	FindQuotaExceeded(ctx context.Context) ([]*entity.APIKey, error)
	FindNextAvailable(ctx context.Context, service string) (*entity.APIKey, error)
	ResetExpiredQuotas(ctx context.Context) error
}
