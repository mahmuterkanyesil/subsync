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
	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisURL},
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
	var payload map[string]string
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	engPath, ok := payload["eng_path"]
	if !ok {
		log.Printf("missing eng_path in payload")
		return nil
	}

	return s.translationUseCase.Translate(ctx, engPath)
}
