package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/replay"
)

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "immutable raw batch root")
	databaseURL := flag.String("database-url", os.Getenv("DBA_DATABASE_URL"), "PostgreSQL connection URL")
	watchInterval := flag.Duration("watch-interval", 0, "continuously normalize new batches at this interval; zero runs once")
	checkpointPath := flag.String("checkpoint", "", "durable manifest checkpoint path; defaults under raw-dir")
	flag.Parse()
	if *databaseURL == "" {
		log.Fatal("database URL is required")
	}
	if *checkpointPath == "" {
		*checkpointPath = filepath.Join(*rawRoot, "normalize.checkpoint")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	store, err := postgres.Open(ctx, *databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	tracker, err := replay.NewCheckpointTracker(*checkpointPath)
	if err != nil {
		log.Fatal(err)
	}
	run := func() {
		stats, runErr := tracker.Run(ctx, *rawRoot, store)
		if runErr != nil {
			log.Printf("normalization failed: %v", runErr)
			return
		}
		if stats.Batches > 0 || *watchInterval == 0 {
			log.Printf("normalization complete: batches=%d events=%d", stats.Batches, stats.Events)
		}
	}
	run()
	if *watchInterval <= 0 {
		return
	}
	ticker := time.NewTicker(*watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
