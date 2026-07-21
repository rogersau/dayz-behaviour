package ranking_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/ranking"
)

func TestOneSessionCannotCreateHighPriority(t *testing.T) {
	result := features.ReadinessResult{
		HiddenTrials: 100, ReadinessLiftLowerBound: 0.9,
		IndependentSessionCount: 1, IndependentEncounterCount: 20, IndependentTargetCount: 20,
	}
	decision := ranking.Apply(result, ranking.DefaultGates())
	if decision.Tier != ranking.InsufficientData {
		t.Fatalf("decision = %+v", decision)
	}
}
