package port

import "context"

type SRTBlock struct {
	Index     int
	Timestamp string
	Text      string
}

type TranslationProvider interface {
	TranslateBatch(ctx context.Context, blocks []SRTBlock, key string) ([]SRTBlock, error)
}

type VideoProcessor interface {
	EnsureEngSubtitle(ctx context.Context, videoPath string) (string, error)
	EmbedSubtitle(ctx context.Context, videoPath string, srtPath string) error
	HasTurkishSubtitle(ctx context.Context, videoPath string) (bool, error)
}

type ProgressStore interface {
	Save(ctx context.Context, engPath string, blocks []SRTBlock) error
	Load(ctx context.Context, engPath string) ([]SRTBlock, bool, error)
	Clear(ctx context.Context, engPath string) error
}
