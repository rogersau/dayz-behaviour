package features

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/observations"
)

func TestReadinessAggregatesIndependentPlayerSessions(t *testing.T) {
	input := []observations.Observation{
		{ObserverPlayerSessionID: "s1", ServerSessionID: "server1", EncounterID: "e1", TargetPlayerSessionID: "t1", Independent: true, StrongHiddenEligible: true, OutcomeObserved: true, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA"},
		{ObserverPlayerSessionID: "s2", ServerSessionID: "server2", EncounterID: "e2", TargetPlayerSessionID: "t2", Independent: true, ControlEligible: true, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA"},
	}
	result := EstimateReadinessForSessions("player", []string{"s1", "s2"}, input, 1, 1)
	if result.IndependentSessionCount != 2 || result.HiddenTrials != 1 || result.ControlTrials != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
