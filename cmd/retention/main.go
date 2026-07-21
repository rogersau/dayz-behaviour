package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/retention"
)

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "restricted raw batch root")
	databaseURL := flag.String("database-url", os.Getenv("DBA_DATABASE_URL"), "PostgreSQL connection URL")
	rawAge := flag.Duration("raw-retention", 30*24*time.Hour, "raw retention duration")
	normalizedAge := flag.Duration("normalized-retention", 90*24*time.Hour, "normalized retention duration")
	reviewAge := flag.Duration("review-retention", 365*24*time.Hour, "review retention duration")
	execute := flag.Bool("execute", false, "perform deletion; otherwise report exact database and filesystem counts")
	flag.Parse()
	if *databaseURL == "" {
		log.Fatal("database URL is required")
	}
	now := time.Now().UTC()
	rawBefore, normalizedBefore, reviewBefore := now.Add(-*rawAge), now.Add(-*normalizedAge), now.Add(-*reviewAge)
	rawStats, err := retention.PruneRaw(*rawRoot, rawBefore, *execute)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, *databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	databaseStats, err := store.ApplyRetention(ctx, postgres.RetentionCutoffs{Raw: &rawBefore, Normalized: &normalizedBefore, Review: &reviewBefore}, *execute)
	if err != nil {
		log.Fatal(err)
	}
	data, _ := json.MarshalIndent(map[string]any{"execute": *execute, "raw": rawStats, "database": databaseStats}, "", "  ")
	fmt.Println(string(data))
}
