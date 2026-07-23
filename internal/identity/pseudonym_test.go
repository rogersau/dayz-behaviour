package identity_test

import (
	"os"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/identity"
)

func TestKeyedPseudonymsAreStableAndDomainPrefixed(t *testing.T) {
	t.Setenv("DBA_PSEUDONYM_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DBA_PSEUDONYM_KEY_ID", "test-key-v1")

	first := identity.DurableID("76561198000000000")
	second := identity.DurableID("76561198000000000")
	if first != second || len(first) != 67 || first[:3] != "dp_" {
		t.Fatalf("unexpected durable pseudonym %q", first)
	}
	if identity.SessionID("session")[:3] != "ps_" {
		t.Fatal("session pseudonym did not use ps_ domain prefix")
	}
}

func TestKeyedPseudonymRequiresKeyID(t *testing.T) {
	t.Setenv("DBA_PSEUDONYM_SECRET", "0123456789abcdef0123456789abcdef")
	_ = os.Unsetenv("DBA_PSEUDONYM_KEY_ID")
	if _, err := identity.CurrentPolicy(); err == nil {
		t.Fatal("expected missing key ID to fail")
	}
}

func TestLegacyPolicyIsExplicit(t *testing.T) {
	_ = os.Unsetenv("DBA_PSEUDONYM_SECRET")
	_ = os.Unsetenv("DBA_PSEUDONYM_KEY_ID")
	policy, err := identity.CurrentPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if policy.Version != identity.LegacyPolicyVersion || policy.KeyID != identity.LegacyKeyID {
		t.Fatalf("unexpected legacy policy: %+v", policy)
	}
}
