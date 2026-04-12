package port

import "context"

// TranslateTask, translate_srt görevinin tip-güvenli payload'ıdır.
type TranslateTask struct {
	EngPath   string `json:"eng_path"`
	VideoPath string `json:"video_path"`
}

type TaskQueue interface {
	Enqueue(ctx context.Context, taskName string, payload any) error
}
