package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestRobustOcclusionAcceptsValidatedFirstPersonHead(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"observer_player_id":             "observer",
		"target_player_id":               "target",
		"classification":                 "ROBUSTLY_OCCLUDED",
		"observer_origin_mode":           "VALIDATED_FIRST_PERSON_HEAD",
		"sampling_stream":                "random_opportunity",
		"sampling_policy_version":        "dual-stream-v1",
		"sampling_reason":                "prospective_random_trigger",
		"risk_set_definition":            "alive_nonself_within_radius",
		"scheduler_load_state":           "normal",
		"visibility_policy_version":      "validated-first-person-head-v1",
		"observer_inclusion_probability": 1.0,
		"target_inclusion_probability":   1.0,
		"queue_admission_probability":    1.0,
		"observer_eligible_count":        2,
		"target_eligible_count":          1,
		"probe_started_ms":               1_000,
		"probe_completed_ms":             1_001,
		"occlusion_duration_ms":          250,
		"visibility_validation_id":       "fixture-v1",
	})
	event := schema.Event{EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, ServerSequence: 1, ServerTimeMS: 1_000, Payload: payload}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
}
