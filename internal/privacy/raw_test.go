package privacy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestScrubRawRemovesSubjectEventsOnly(t *testing.T) {
	root := t.TempDir()
	batch := schema.Batch{SchemaVersion: 1, ServerID: "s", ServerSessionID: "ss", BatchSequence: 1, Events: []schema.Event{
		{EventType: "PLAYER_SNAPSHOT", Source: schema.SourceServer, ServerSequence: 1, PlayerSessionID: "ss:delete-me", Payload: json.RawMessage(`{}`)},
		{EventType: "PLAYER_SNAPSHOT", Source: schema.SourceServer, ServerSequence: 2, PlayerSessionID: "ss:keep-me", Payload: json.RawMessage(`{}`)},
	}}
	data, _ := json.Marshal(batch)
	path := filepath.Join(root, "batch.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	stats, err := ScrubRaw(root, "delete-me", true)
	if err != nil {
		t.Fatal(err)
	}
	if stats.EventsRemoved != 1 || stats.FilesRewritten != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(updated, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 || batch.Events[0].PlayerSessionID != "ss:keep-me" {
		t.Fatalf("unexpected events: %+v", batch.Events)
	}
}
