package identity_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/identity"
)

func TestPlayerIdentitiesRemainDirect(t *testing.T) {
	const steamID = "76561198000000000"
	const sessionID = "server-session:12:" + steamID
	if got := identity.DurableID(steamID); got != steamID {
		t.Fatalf("durable identity = %q, want %q", got, steamID)
	}
	if got := identity.SessionID(sessionID); got != sessionID {
		t.Fatalf("session identity = %q, want %q", got, sessionID)
	}
}

func TestDirectIdentityPolicyIsExplicit(t *testing.T) {
	policy, err := identity.CurrentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.Version != identity.DirectPolicyVersion || policy.KeyID != identity.DirectKeyID {
		t.Fatalf("unexpected direct identity policy: %+v", policy)
	}
}

func TestDigestStillProducesOpaqueInternalIDs(t *testing.T) {
	first := identity.MustDigest("record-material")
	second := identity.MustDigest("record-material")
	if first != second || len(first) != 64 {
		t.Fatalf("unexpected digest %q", first)
	}
}
