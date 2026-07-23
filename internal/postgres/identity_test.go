package postgres

import (
	"encoding/json"
	"testing"
)

func TestNormalizePayloadPreservesDirectIdentityFields(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"observer_player_session_id": "server:1:76561198000000000",
		"target_player_id":           "76561198000000001",
		"source_player_id":           "76561198000000002",
	})
	_, fields, err := normalizePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := stringField(fields, "observer_player_session_id"); got != "server:1:76561198000000000" {
		t.Fatalf("observer session = %q", got)
	}
	if got := stringField(fields, "target_player_id"); got != "76561198000000001" {
		t.Fatalf("target player = %q", got)
	}
	if got := stringField(fields, "source_player_id"); got != "76561198000000002" {
		t.Fatalf("source player = %q", got)
	}
}
