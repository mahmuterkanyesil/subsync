package main

import (
	"context"
	"database/sql"
	"log"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/queue/asynq"
	"subsync/internal/infrastructure/adapter/translation/gemini"
	"subsync/pkg/config"
	"subsync/pkg/progress"

	_ "modernc.org/sqlite"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	db, err := sql.Open("sqlite", cfg.StateDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	apiKeyRepo := sqlite.NewSQLiteAPIKeyRepository(db)

	translator, err := gemini.NewGeminiTranslator(ctx)
	if err != nil {
		log.Fatal(err)
	}

	progressStore := progress.NewFileProgressStore(cfg.ProgressDir)

	translationService := service.NewTranslationService(subtitleRepo, apiKeyRepo, translator, progressStore)

	workerServer := asynq.NewAsynqWorkerServer(cfg.RedisURL, cfg.WorkerConcurrency, translationService)

	log.Println("worker started")
	if err := workerServer.Start(); err != nil {
		log.Fatal(err)
	}
}
