package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/queue/asynq"
	"subsync/internal/infrastructure/adapter/video/ffmpeg"
	"subsync/pkg/config"
	"subsync/pkg/logger"
)

func main() {
	cfg := config.Load()
	// init logger early
	logger.Init()

	db, err := sqlite.Open(cfg.StateDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	watchDirRepo := sqlite.NewSQLiteWatchDirRepository(db)
	settingsRepo := sqlite.NewSQLiteAppSettingsRepository(db)
	videoProcessor := ffmpeg.NewFFmpegProcessor()
	taskQueue := asynq.NewAsynqTaskQueue(cfg.RedisURL)

	scanner := service.NewScanningService(subtitleRepo, videoProcessor, taskQueue, cfg.WatchDirs, watchDirRepo, settingsRepo)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("agent started")
	for {
		if err := scanner.Scan(ctx); err != nil {
			logger.Error("scan error: %v", err)
		}
		select {
		case <-ctx.Done():
			logger.Info("agent shutting down")
			return
		case <-time.After(time.Duration(cfg.ScanIntervalSec) * time.Second):
		}
	}
}
