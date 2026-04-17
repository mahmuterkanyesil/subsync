package port

import (
	"context"
	"time"
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
	Delete(ctx context.Context, engPath string) error
}

type WatchDirRepository interface {
	FindAll(ctx context.Context) ([]*entity.WatchDir, error)
	FindEnabled(ctx context.Context) ([]string, error)
	FindByID(ctx context.Context, id int) (*entity.WatchDir, error)
	Save(ctx context.Context, w *entity.WatchDir) error
	Delete(ctx context.Context, id int) error
}

type APIKeyRepository interface {
	Save(ctx context.Context, k *entity.APIKey) error
	FindByID(ctx context.Context, id int) (*entity.APIKey, error)
	FindNextAvailable(ctx context.Context, service string) (*entity.APIKey, error)
	FindEarliestQuotaReset(ctx context.Context, service string) (*time.Time, error)
	ResetExpiredQuotas(ctx context.Context) error
	FindAll(ctx context.Context) ([]*entity.APIKey, error)
	Delete(ctx context.Context, id int) error
}
