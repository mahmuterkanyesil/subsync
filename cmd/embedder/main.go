package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subsync/internal/core/application/service"
	eventadapter "subsync/internal/infrastructure/adapter/event"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/video/ffmpeg"
	"subsync/pkg/config"
	"subsync/pkg/logger"
)

func main() {
	cfg := config.Load()
	logger.Init()

	db, err := sqlite.Open(cfg.StateDBPath)
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
	defer db.Close()

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	settingsRepo := sqlite.NewSQLiteAppSettingsRepository(db)
	videoProcessor := ffmpeg.NewFFmpegProcessor()

	embeddingService := service.NewEmbeddingService(
		subtitleRepo,
		videoProcessor,
		eventadapter.NewLogEventPublisher(),
		settingsRepo,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("embedder started")
	for {
		if err := embeddingService.EmbedPending(ctx); err != nil {
			logger.Warn("embed error: %v", err)
		}
		select {
		case <-ctx.Done():
			logger.Info("embedder shutting down")
			return
		case <-time.After(time.Duration(cfg.EmbedIntervalSec) * time.Second):
		}
	}
}
