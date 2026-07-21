package features

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/observations"
)

func TestConditionalLogitFindsPositiveMatchedEffect(t *testing.T) {
	var input []observations.Observation
	for stratum := 0; stratum < 12; stratum++ {
		base := observations.Observation{ObserverPlayerSessionID: "p", ServerID: "s", MapID: "m", AreaCell: "a", DistanceBand: "25-50", ObserverMovementBand: "SLOW", BaselineWeaponState: "LOWERED", CameraMode: "FIRST_PERSON", ServerPopulationBand: "10-29", CueClass: "UNEXPLAINED_IN_CAPTURED_DATA", SamplingPolicyVersion: "v1", Independent: true}
		hidden := base
		hidden.ObservationID = intString(stratum) + "h"
		hidden.StrongHiddenEligible = true
		hidden.OutcomeObserved = stratum%4 != 0
		control := base
		control.ObservationID = intString(stratum) + "c"
		control.ControlEligible = true
		control.OutcomeObserved = stratum%4 == 0
		input = append(input, hidden, control)
	}
	strata := BuildMatchedStrata(input)
	result := FitConditionalLogit("p", strata)
	if !result.Converged || result.Separated || result.OddsRatio <= 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestConditionalLogitSuppressesSeparation(t *testing.T) {
	strata := []MatchedStratum{{PlayerSessionID: "p", Observations: []observations.Observation{
		{StrongHiddenEligible: true, OutcomeObserved: true}, {ControlEligible: true, OutcomeObserved: false},
	}}}
	result := FitConditionalLogit("p", strata)
	if !result.Separated {
		t.Fatalf("expected separation: %+v", result)
	}
}
