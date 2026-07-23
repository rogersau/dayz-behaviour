package observations_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestBuilderDoesNotJoinEventsAcrossServerSessions(t *testing.T) {
	visibilityPayload, _ := json.Marshal(map[string]any{
		"target_player_id":             "target",
		"classification":               "EXPOSED",
		"observer_origin_mode":         "PLAYER_HEAD_APPROXIMATION",
		"sampling_stream":              "random_opportunity",
		"sampling_policy_version":      "v1",
		"target_inclusion_probability": 1.0,
		"queue_admission_probability":  1.0,
		"probe_started_ms":             1_000,
	})
	edgePayload, _ := json.Marshal(map[string]any{"sampling_reason": "WEAPON_RAISED"})

	batches := []schema.Batch{
		{
			SchemaVersion: 1, ServerID: "server", ServerSessionID: "session-a", BatchSequence: 1,
			Events: []schema.Event{{EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, SourceEventID: "visibility-a", ServerSequence: 1, ServerTimeMS: 1_000, PlayerSessionID: "observer", Payload: visibilityPayload}},
		},
		{
			SchemaVersion: 1, ServerID: "server", ServerSessionID: "session-b", BatchSequence: 1,
			Events: []schema.Event{{EventType: "DECISION_EDGE", Source: schema.SourceClient, SourceAuthority: schema.AuthorityClient, SourceEventID: "edge-b", ServerSequence: 1, ServerTimeMS: 1_500, PlayerSessionID: "observer", Payload: edgePayload}},
		},
	}

	got, err := observations.Build(batches, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].OutcomeObserved {
		t.Fatalf("cross-session decision edge was joined to observation: %+v", got[0])
	}

	reversed := []schema.Batch{batches[1], batches[0]}
	gotReversed, err := observations.Build(reversed, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, gotReversed) {
		t.Fatalf("result depends on raw file discovery order\nforward=%+v\nreverse=%+v", got, gotReversed)
	}
}

func TestBuilderAcceptsValidatedFirstPersonHeadOrigin(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"target_player_id":             "target",
		"classification":               "ROBUSTLY_OCCLUDED",
		"observer_origin_mode":         "VALIDATED_FIRST_PERSON_HEAD",
		"sampling_stream":              "random_opportunity",
		"sampling_policy_version":      "v1",
		"target_inclusion_probability": 1.0,
		"queue_admission_probability":  1.0,
		"queue_delay_ms":               1,
		"probe_started_ms":             1_000,
		"occlusion_duration_ms":        250,
		"visibility_validation_id":     "fixture-v1",
	})
	batch := schema.Batch{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1, Events: []schema.Event{{
		EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer,
		SourceEventID: "visibility", ServerSequence: 1, ServerTimeMS: 1_000, PlayerSessionID: "observer", Payload: payload,
	}}}
	got, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].StrongHiddenEligible {
		t.Fatalf("observation = %+v", got)
	}
}
