package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/replay"
)

const confirmation = "REBUILD_WITH_DIRECT_IDENTITIES"

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "restricted raw batch root")
	databaseURL := flag.String("database-url", os.Getenv("DBA_DATABASE_URL"), "PostgreSQL connection URL")
	execute := flag.Bool("execute", false, "clear derived data and replay raw batches")
	confirm := flag.String("confirm", "", "required confirmation phrase when executing")
	flag.Parse()
	if *databaseURL == "" {
		log.Fatal("database URL is required")
	}

	ctx := context.Background()
	store, err := postgres.Open(ctx, *databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	scope, err := store.DirectIdentityRebuildScope(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !*execute {
		printJSON(map[string]any{
			"mode": "dry-run", "rows_to_rebuild": scope,
			"preserved": []string{"restricted raw batch files", "raw_batches metadata", "privacy_audit_log"},
			"execute_with": "-execute -confirm " + confirmation,
		})
		return
	}
	if *confirm != confirmation {
		log.Fatalf("-confirm must equal %s", confirmation)
	}

	cleared, err := store.ResetForDirectIdentities(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	replayStats, err := replay.Run(ctx, *rawRoot, store)
	if err != nil {
		log.Fatal(err)
	}
	checkpoint := filepath.Join(*rawRoot, "normalize.checkpoint")
	checkpointRemoved := false
	if err := os.Remove(checkpoint); err == nil {
		checkpointRemoved = true
	} else if !os.IsNotExist(err) {
		log.Printf("warning: remove normalization checkpoint: %v", err)
	}

	printJSON(map[string]any{
		"mode": "executed", "cleared_rows": cleared,
		"replayed_batches": replayStats.Batches, "replayed_events": replayStats.Events,
		"normalization_checkpoint_removed": checkpointRemoved,
		"next_step": "run cmd/analyse to rebuild features, rankings, and review cases",
	})
}

func printJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
