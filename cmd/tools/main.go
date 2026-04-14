package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"subsync/internal/core/domain/valueobject"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"
	"subsync/pkg/config"
	"subsync/pkg/logger"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tools <subcommand>")
		fmt.Println("  requeue_failed   - transition all error entries to queued")
		fmt.Println("  embed_existing   - log StatusDone entries that have .tr.srt ready")
		os.Exit(1)
	}

	cfg := config.Load()
	logger.Init()

	db, err := sql.Open("sqlite", cfg.StateDBPath)
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.Migrate(db); err != nil {
		log.Fatal(err)
	}

	subtitleRepo := sqlite.NewSQLiteSubtitleRepository(db)
	ctx := context.Background()

	switch os.Args[1] {
	case "requeue_failed":
		entries, err := subtitleRepo.FindByStatus(ctx, valueobject.StatusError)
		if err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		requeued := 0
		for _, s := range entries {
			if err := s.TransitionTo(valueobject.StatusQueued); err != nil {
				log.Printf("skip %s: %v", s.EngPath(), err)
				continue
			}
			if err := subtitleRepo.Save(ctx, s); err != nil {
				logger.Warn("save error %s: %v", s.EngPath(), err)
				continue
			}
			requeued++
		}
		fmt.Printf("requeued %d/%d failed entries\n", requeued, len(entries))

	case "embed_existing":
		entries, err := subtitleRepo.FindByStatus(ctx, valueobject.StatusDone)
		if err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		ready := 0
		for _, s := range entries {
			trPath := strings.TrimSuffix(s.EngPath(), filepath.Ext(s.EngPath())) + ".tr.srt"
			if _, err := os.Stat(trPath); err != nil {
				continue
			}
			// Re-save to ensure the embedder picks it up on next cycle
			if err := subtitleRepo.Save(ctx, s); err != nil {
				logger.Warn("save error %s: %v", s.EngPath(), err)
				continue
			}
			logger.Info("ready for embed: %s", s.EngPath())
			ready++
		}
		fmt.Printf("%d entries ready for embed (embedder will pick up on next cycle)\n", ready)

	default:
		log.Fatalf("unknown subcommand: %s", os.Args[1])
	}
}
