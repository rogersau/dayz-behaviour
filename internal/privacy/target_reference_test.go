package privacy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestScrubRawRemovesTargetOnlySessionReference(t *testing.T) {
	root := t.TempDir()
	targetPayload, _ := json.Marshal(map[string]any{
		"observer_player_id":         "observer",
		"target_player_id":           "delete-me",
		"target_player_session_id":   "server-session:42:delete-me",
		"observer_player_session_id": "server-session:1:observer",
	})
	keepPayload, _ := json.Marshal(map[string]any{"target_player_id": "keep-me"})
	batch := schema.Batch{SchemaVersion: 1, ServerID: "s", ServerSessionID: "server-session", BatchSequence: 1, Events: []schema.Event{
		{EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, ServerSequence: 1, PlayerSessionID: "server-session:1:observer", Payload: targetPayload},
		{EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, ServerSequence: 2, PlayerSessionID: "server-session:1:observer", Payload: keepPayload},
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
	if len(batch.Events) != 1 || batch.Events[0].ServerSequence != 2 {
		t.Fatalf("unexpected remaining events: %+v", batch.Events)
	}
}
