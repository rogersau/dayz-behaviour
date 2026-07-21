package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/rogersau/dayz-behaviour/internal/replay"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func main() {
	rawRoot := flag.String("raw-dir", "./data/raw", "immutable raw batch root")
	validateOnly := flag.Bool("validate-only", false, "validate deterministically without writing batches")
	flag.Parse()

	sink := replay.SinkFunc(func(_ context.Context, batch schema.Batch) error {
		if *validateOnly {
			return nil
		}
		data, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	})

	stats, err := replay.Run(context.Background(), *rawRoot, sink)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("replay complete: batches=%d events=%d", stats.Batches, stats.Events)
}
