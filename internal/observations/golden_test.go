package observations_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestGoldenReplayIsDeterministic(t *testing.T) {
	data, err := os.ReadFile("testdata/golden-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var batch schema.Batch
	if err := json.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	if err := batch.Validate(); err != nil {
		t.Fatal(err)
	}
	first, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	second, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("fixed replay produced divergent observations")
	}
	if len(first) != 2 || !first[0].StrongHiddenEligible || !first[0].OutcomeObserved || !first[1].ControlEligible || first[1].OutcomeObserved {
		t.Fatalf("unexpected golden observations: %+v", first)
	}
	result := features.EstimateReadiness("golden-session:1:observer", first, 1, 1)
	if result.HiddenSuccesses != 1 || result.HiddenTrials != 1 || result.ControlSuccesses != 0 || result.ControlTrials != 1 || result.ReadinessLift <= 0 {
		t.Fatalf("unexpected golden feature result: %+v", result)
	}
}
