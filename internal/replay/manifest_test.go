package replay_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/replay"
	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestCheckpointTrackerConsumesOnlyNewManifestEntries(t *testing.T) {
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
	var sequences []uint64
	sink := replay.SinkFunc(func(_ context.Context, batch schema.Batch) error {
		sequences = append(sequences, batch.BatchSequence)
		return nil
	})
	stats, err := tracker.Run(context.Background(), root, sink)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Batches != 1 {
		t.Fatalf("initial batches = %d, want 1", stats.Batches)
	}
	if err := store.Put(manifestBatch(2)); err != nil {
		t.Fatal(err)
	}
	stats, err = tracker.Run(context.Background(), root, sink)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Batches != 1 || sequences[len(sequences)-1] != 2 {
		t.Fatalf("incremental stats=%+v sequences=%v", stats, sequences)
	}

	restarted, err := replay.NewCheckpointTracker(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(manifestBatch(3)); err != nil {
		t.Fatal(err)
	}
	stats, err = restarted.Run(context.Background(), root, sink)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Batches != 1 || sequences[len(sequences)-1] != 3 {
		t.Fatalf("restart stats=%+v sequences=%v", stats, sequences)
	}
}

func manifestBatch(sequence uint64) schema.Batch {
	return schema.Batch{
		SchemaVersion:   1,
		ServerID:       "server",
		ServerSessionID: "session",
		BatchSequence:  sequence,
		ServerTimeMS:   int64(sequence),
		Events: []schema.Event{{
			EventType:       "PLAYER_SNAPSHOT",
			Source:          schema.SourceServer,
			SourceAuthority: schema.AuthorityServer,
			ServerSequence:  sequence,
			ServerTimeMS:    int64(sequence),
		}},
	}
}
