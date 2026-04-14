package main

import (
	"database/sql"
	"os"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/queue/asynq"
	"subsync/internal/infrastructure/adapter/translation/gemini"
	"subsync/pkg/config"
	"subsync/pkg/logger"
	"subsync/pkg/progress"

	_ "modernc.org/sqlite"
)

func main() {
	cfg := config.Load()
	logger.Init()

	db, err := sql.Open("sqlite", cfg.StateDBPath)
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	apiKeyRepo := sqlite.NewSQLiteAPIKeyRepository(db)

	translator := gemini.NewGeminiTranslator()

	progressStore := progress.NewFileProgressStore(cfg.ProgressDir)

	translationService := service.NewTranslationService(
		subtitleRepo,
		apiKeyRepo,
		translator,
		progressStore,
		nil, // EventPublisher — ileride bağlanabilir
		cfg.BatchSize,
	)

	workerServer := asynq.NewAsynqWorkerServer(cfg.RedisURL, cfg.WorkerConcurrency, translationService)

	logger.Info("worker started")
	if err := workerServer.Start(); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
}
