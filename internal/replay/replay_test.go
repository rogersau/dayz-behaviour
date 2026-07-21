package replay_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/replay"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestRunIsDeterministic(t *testing.T) {
	root := t.TempDir()
	writeBatch(t, root, "server-b", 2)
	writeBatch(t, root, "server-a", 2)
	writeBatch(t, root, "server-a", 1)

	var order []string
	stats, err := replay.Run(context.Background(), root, replay.SinkFunc(func(_ context.Context, batch schema.Batch) error {
		order = append(order, batch.ServerID+":"+string(rune('0'+batch.BatchSequence)))
		return nil
	}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Batches != 3 || stats.Events != 3 {
		t.Fatalf("stats = %+v", stats)
	}
	want := []string{"server-a:1", "server-a:2", "server-b:2"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestTrackerOnlyReplaysNewBatches(t *testing.T) {
	root := t.TempDir()
	writeBatch(t, root, "server-a", 1)

	tracker := replay.NewTracker()
	var accepted int
	sink := replay.SinkFunc(func(_ context.Context, _ schema.Batch) error {
		accepted++
		return nil
	})
	first, err := tracker.Run(context.Background(), root, sink)
	if err != nil {
		t.Fatal(err)
	}
	second, err := tracker.Run(context.Background(), root, sink)
	if err != nil {
		t.Fatal(err)
	}
	writeBatch(t, root, "server-a", 2)
	third, err := tracker.Run(context.Background(), root, sink)
	if err != nil {
		t.Fatal(err)
	}

	if first.Batches != 1 || second.Batches != 0 || third.Batches != 1 || accepted != 2 {
		t.Fatalf("stats = (%+v, %+v, %+v), accepted = %d", first, second, third, accepted)
	}
}

func writeBatch(t *testing.T, root, serverID string, sequence uint64) {
	t.Helper()
	batch := schema.Batch{
		SchemaVersion:   schema.Version1,
		ServerID:        serverID,
		ServerSessionID: "session",
		BatchSequence:   sequence,
		ServerTimeMS:    1,
		Events: []schema.Event{{
			EventType:      "PLAYER_SNAPSHOT",
			Source:         schema.SourceServer,
			ServerSequence: sequence,
			ServerTimeMS:   1,
		}},
	}
	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, serverID, "session")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(serverID)+"-"+string(rune('0'+sequence))+".json"), data, 0o640); err != nil {
		t.Fatal(err)
	}
}
