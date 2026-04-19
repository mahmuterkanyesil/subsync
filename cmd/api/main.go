package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/queue/asynq"
	"subsync/internal/infrastructure/adapter/rest/gin"
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
	apiKeyRepo := sqlite.NewSQLiteAPIKeyRepository(db)
	watchDirRepo := sqlite.NewSQLiteWatchDirRepository(db)
	taskQueue := asynq.NewAsynqTaskQueue(cfg.RedisURL)

	statsService := service.NewStatsService(subtitleRepo, apiKeyRepo, watchDirRepo, taskQueue)

	server := gin.NewHTTPServer(statsService, cfg.APIPort)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("api started on port %s", cfg.APIPort)
	if err := server.Start(ctx); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
}
