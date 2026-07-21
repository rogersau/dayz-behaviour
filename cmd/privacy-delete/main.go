package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/privacy"
)

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "restricted raw batch root")
	databaseURL := flag.String("database-url", os.Getenv("DBA_DATABASE_URL"), "PostgreSQL connection URL")
	playerID := flag.String("player-id", "", "durable DayZ identity to delete")
	actor := flag.String("actor", "", "operator performing the deletion")
	reason := flag.String("reason", "", "deletion reason or request reference")
	execute := flag.Bool("execute", false, "perform deletion; otherwise report a dry run")
	flag.Parse()
	if *playerID == "" {
		log.Fatal("player-id is required")
	}
	stats, err := privacy.ScrubRaw(*rawRoot, *playerID, *execute)
	if err != nil {
		log.Fatal(err)
	}
	if !*execute {
		printJSON(map[string]any{"mode": "dry-run", "raw": stats})
		return
	}
	if *databaseURL == "" || *actor == "" || *reason == "" {
		log.Fatal("database-url, actor and reason are required with -execute")
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, *databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	refs := make([]postgres.BatchRef, 0, len(stats.AffectedBatches))
	for _, batch := range stats.AffectedBatches {
		refs = append(refs, postgres.BatchRef{ServerID: batch.ServerID, ServerSessionID: batch.ServerSessionID, BatchSequence: batch.BatchSequence})
	}
	counts, err := store.DeletePlayer(ctx, *playerID, *actor, *reason, refs)
	if err != nil {
		log.Fatal(err)
	}
	printJSON(map[string]any{"mode": "executed", "raw": stats, "database": counts})
}

func printJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
