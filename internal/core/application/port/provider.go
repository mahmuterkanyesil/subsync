package port

import (
	"context"
	"errors"
	"subsync/internal/core/domain/valueobject"
)

// SRTBlock, altyazı blok işlemlerinde kullanılan port tipidir.
// Domain'deki valueobject.SRTBlock'a type alias olarak tanımlanmıştır.
type SRTBlock = valueobject.SRTBlock

// Video işleme sentinel hataları — tüm katmanlar bu hataları kullanır.
var (
	ErrVideoNotFound  = errors.New("video_not_found")
	ErrFFmpegNotFound = errors.New("ffmpeg_not_found")
	ErrEngSrtTooLarge = errors.New("eng_too_large")
	ErrTrSrtNotFound  = errors.New("tr_srt_not_found")
	ErrFFmpegFailed   = errors.New("ffmpeg_failed")
	ErrOutputTooSmall = errors.New("output_too_small")
	ErrNoEngStream    = errors.New("no_suitable_english_stream")
)

type TranslationProvider interface {
	TranslateBatch(ctx context.Context, blocks []SRTBlock, keyValue, model, targetLang string) ([]SRTBlock, error)
}

type VideoProcessor interface {
	EnsureEngSubtitle(ctx context.Context, videoPath string) (string, error)
	EmbedSubtitle(ctx context.Context, videoPath, srtPath, langFFmpegCode, langNameEN string) error
	HasTargetSubtitle(ctx context.Context, videoPath, langFFmpegCode string) (bool, error)
}

type ProgressStore interface {
	Save(ctx context.Context, engPath string, blocks []SRTBlock) error
	Load(ctx context.Context, engPath string) ([]SRTBlock, bool, error)
	Clear(ctx context.Context, engPath string) error
}
