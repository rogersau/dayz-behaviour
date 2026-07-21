package features_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/observations"
)

func TestSparseReadinessShrinksTowardPrior(t *testing.T) {
	input := []observations.Observation{{
		ObserverPlayerSessionID: "player", ServerSessionID: "session", EncounterID: "encounter",
		TargetPlayerSessionID: "target", Independent: true, StrongHiddenEligible: true,
		OutcomeObserved: true, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA",
	}}
	result := features.EstimateReadiness("player", input, 5, 5)
	if result.HiddenPosteriorMean >= 0.7 {
		t.Fatalf("sparse posterior was not shrunk: %+v", result)
	}
	if result.HiddenLowerBound > 0.5 {
		t.Fatalf("sparse lower bound too strong: %+v", result)
	}
}
