package features_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/observations"
)

func TestReadinessCountsDurableTargetOnceAcrossSessions(t *testing.T) {
	input := []observations.Observation{
		{ObserverPlayerSessionID: "observer-1", TargetPlayerSessionID: "target-session-1", TargetIdentityKey: "target-stable", EncounterID: "encounter-1", Independent: true, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA", StrongHiddenEligible: true},
		{ObserverPlayerSessionID: "observer-2", TargetPlayerSessionID: "target-session-2", TargetIdentityKey: "target-stable", EncounterID: "encounter-2", Independent: true, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA", StrongHiddenEligible: true},
	}
	result := features.EstimateReadinessForSessions("observer", []string{"observer-1", "observer-2"}, input, 1, 1)
	if result.IndependentTargetCount != 1 {
		t.Fatalf("independent target count = %d, want 1", result.IndependentTargetCount)
	}
}
