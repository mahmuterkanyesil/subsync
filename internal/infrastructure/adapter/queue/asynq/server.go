package asynq

import (
	"context"
	"encoding/json"
	"log"
	"subsync/internal/core/application/port"

	"github.com/hibiken/asynq"
)

type AsynqWorkerServer struct {
	server             *asynq.Server
	translationUseCase port.TranslationUseCase
}

func NewAsynqWorkerServer(redisURL string, concurrency int, translationUseCase port.TranslationUseCase) *AsynqWorkerServer {
	opt := parseRedisURL(redisURL)
	server := asynq.NewServer(
		opt,
		asynq.Config{Concurrency: concurrency},
	)
	return &AsynqWorkerServer{
		server:             server,
		translationUseCase: translationUseCase,
	}
}

func (s *AsynqWorkerServer) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc("translate_srt", s.handleTranslate)
	return s.server.Run(mux)
}

func (s *AsynqWorkerServer) handleTranslate(ctx context.Context, task *asynq.Task) error {
	var payload port.TranslateTask
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		log.Printf("invalid translate_srt payload: %v", err)
		return nil
	}
	if payload.EngPath == "" {
		log.Printf("translate_srt: missing eng_path")
		return nil
	}
	return s.translationUseCase.Translate(ctx, payload.EngPath)
}
