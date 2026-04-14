package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/queue/asynq"
	"subsync/internal/infrastructure/adapter/video/ffmpeg"
	"subsync/pkg/config"
	"subsync/pkg/logger"

	_ "modernc.org/sqlite"
)

func main() {
	cfg := config.Load()
	// init logger early
	logger.Init()

	db, err := sql.Open("sqlite", cfg.StateDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		log.Fatal(err)
	}

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	watchDirRepo := sqlite.NewSQLiteWatchDirRepository(db)
	videoProcessor := ffmpeg.NewFFmpegProcessor()
	taskQueue := asynq.NewAsynqTaskQueue(cfg.RedisURL)

	scanner := service.NewScanningService(subtitleRepo, videoProcessor, taskQueue, cfg.WatchDirs, watchDirRepo)

	logger.Info("agent started")
	for {
		if err := scanner.Scan(context.Background()); err != nil {
			logger.Error("scan error: %v", err)
		}
		time.Sleep(time.Duration(cfg.ScanIntervalSec) * time.Second)
	}
}
