package postgres

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/identity"
)

func TestNormalizePayloadUsesSessionAndDurableIdentityDomains(t *testing.T) {
	t.Setenv("DBA_PSEUDONYM_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DBA_PSEUDONYM_KEY_ID", "test-v1")
	raw, _ := json.Marshal(map[string]any{
		"observer_player_session_id": "server:1:76561198000000000",
		"target_player_id":           "76561198000000001",
		"source_player_id":           "76561198000000002",
	})
	_, fields, err := normalizePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := stringField(fields, "observer_player_session_id"); got != identity.SessionID("server:1:76561198000000000") {
		t.Fatalf("observer session = %q", got)
	}
	if got := stringField(fields, "target_player_id"); got != identity.DurableID("76561198000000001") {
		t.Fatalf("target player = %q", got)
	}
	if got := stringField(fields, "source_player_id"); got != identity.DurableID("76561198000000002") {
		t.Fatalf("source player = %q", got)
	}
}

func TestDurableAndSessionPseudonymsShareConfiguredPolicy(t *testing.T) {
	t.Setenv("DBA_PSEUDONYM_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DBA_PSEUDONYM_KEY_ID", "test-v1")
	if got, want := pseudonymousDurableID("player"), identity.DurableID("player"); got != want {
		t.Fatalf("durable pseudonym = %q, want %q", got, want)
	}
	if got, want := pseudonymousSessionID("session"), identity.SessionID("session"); got != want {
		t.Fatalf("session pseudonym = %q, want %q", got, want)
	}
}
