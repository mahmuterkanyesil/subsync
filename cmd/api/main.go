package main

import (
	"database/sql"
	"log"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/queue/asynq"
	"subsync/internal/infrastructure/adapter/rest/gin"
	"subsync/pkg/config"

	_ "modernc.org/sqlite"
)

func main() {
	cfg := config.Load()

	db, err := sql.Open("sqlite", cfg.StateDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		log.Fatal(err)
	}

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	apiKeyRepo := sqlite.NewSQLiteAPIKeyRepository(db)
	taskQueue := asynq.NewAsynqTaskQueue(cfg.RedisURL)

	statsService := service.NewStatsService(subtitleRepo, apiKeyRepo, taskQueue)

	server := gin.NewHTTPServer(statsService, cfg.APIPort)

	log.Println("api started on port", cfg.APIPort)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
