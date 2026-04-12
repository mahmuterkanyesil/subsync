package port

import (
	"context"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
)

type SubtitleStats struct {
	Total          int
	Done           int
	Queued         int
	Error          int
	QuotaExhausted int
	Embedded       int
	EmbedFailed    int
}

type SubtitleRepository interface {
	Save(ctx context.Context, s *entity.Subtitle) error
	FindByPath(ctx context.Context, path string) (*entity.Subtitle, error)
	FindAll(ctx context.Context) ([]*entity.Subtitle, error)
	FindPendingEmbed(ctx context.Context) ([]*entity.Subtitle, error)
	Statistics(ctx context.Context) (SubtitleStats, error)
	FindBySxxExx(ctx context.Context, season, episode int) ([]*entity.Subtitle, error)
	FindByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error)
}

type APIKeyRepository interface {
	Save(ctx context.Context, k *entity.APIKey) error
	FindByID(ctx context.Context, id int) (*entity.APIKey, error)
	FindNextAvailable(ctx context.Context, service string) (*entity.APIKey, error)
	ResetExpiredQuotas(ctx context.Context) error
}
