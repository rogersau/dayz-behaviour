package replay_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/replay"
	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestCheckpointTrackerSkipsManifestEntryDeletedByRetention(t *testing.T) {
	root := t.TempDir()
	store, err := storage.NewRawStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(manifestBatch(1)); err != nil {
		t.Fatal(err)
	}
	checkpoint := filepath.Join(root, "normalize.checkpoint")
	tracker, err := replay.NewCheckpointTracker(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.Run(context.Background(), root, replay.SinkFunc(func(context.Context, schema.Batch) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(manifestBatch(2)); err != nil {
		t.Fatal(err)
	}
	batchPath := filepath.Join(root, "server", "session", "00000000000000000002.json")
	if err := os.Remove(batchPath); err != nil {
		t.Fatal(err)
	}
	stats, err := tracker.Run(context.Background(), root, replay.SinkFunc(func(context.Context, schema.Batch) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Batches != 0 || stats.Events != 0 {
		t.Fatalf("deleted batch produced replay stats: %+v", stats)
	}
}
