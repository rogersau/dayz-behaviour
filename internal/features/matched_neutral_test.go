package features_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/observations"
)

func TestMatchedStrataUsesNeutralNotVisibleControl(t *testing.T) {
	base := observations.Observation{
		ObserverPlayerSessionID: "observer", ServerID: "server", MapID: "map", AreaCell: "1:1",
		ObserverMovementBand: "STATIONARY", ObserverStanceID: 0, BaselineWeaponState: "LOWERED",
		CameraMode: "FIRST_PERSON_SERVER_POLICY", ServerPopulationBand: "10-29",
		CueClass: "UNEXPLAINED_IN_CAPTURED_DATA", SamplingPolicyVersion: "v2", Independent: true,
	}
	hidden := base
	hidden.ObservationID = "hidden"
	hidden.StrongHiddenEligible = true
	neutral := base
	neutral.ObservationID = "neutral"
	neutral.ControlEligible = true
	visible := base
	visible.ObservationID = "visible"
	visible.PositiveControlEligible = true

	strata := features.BuildMatchedStrata([]observations.Observation{hidden, neutral, visible})
	if len(strata) != 1 {
		t.Fatalf("strata = %d, want 1", len(strata))
	}
	if len(strata[0].Observations) != 2 {
		t.Fatalf("observations = %d, want hidden plus neutral only", len(strata[0].Observations))
	}
	for _, observation := range strata[0].Observations {
		if observation.ObservationID == "visible" {
			t.Fatal("visible positive control entered the primary matched model")
		}
	}
}
