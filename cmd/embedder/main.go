package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/video/ffmpeg"
	"subsync/pkg/config"
	"subsync/pkg/logger"

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
	videoProcessor := ffmpeg.NewFFmpegProcessor()

	embeddingService := service.NewEmbeddingService(
		subtitleRepo,
		videoProcessor,
		nil, // EventPublisher — ileride bağlanabilir
	)

	logger.Info("embedder started")
	for {
		if err := embeddingService.EmbedPending(context.Background()); err != nil {
			logger.Warn("embed error: %v", err)
		}
		time.Sleep(time.Duration(cfg.EmbedIntervalSec) * time.Second)
	}
}
