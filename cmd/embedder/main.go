package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"subsync/internal/core/application/service"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/internal/infrastructure/adapter/video/ffmpeg"
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
	videoProcessor := ffmpeg.NewFFmpegProcessor()

	embeddingService := service.NewEmbeddingService(subtitleRepo, videoProcessor)

	log.Println("embedder started")
	for {
		if err := embeddingService.EmbedPending(context.Background()); err != nil {
			log.Printf("embed error: %v", err)
		}
		time.Sleep(time.Duration(cfg.EmbedIntervalSec) * time.Second)
	}
}
