package observations_test

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestBuilderPreservesDecisionTimeInterval(t *testing.T) {
	neutral, _ := json.Marshal(map[string]any{
		"classification": "NO_RELEVANT_TARGET", "control_type": "NEUTRAL_NO_RELEVANT_TARGET",
		"observer_origin_mode": "NOT_APPLICABLE", "sampling_stream": "random_opportunity",
		"sampling_policy_version": "v2", "queue_admission_probability": 1.0,
		"target_inclusion_probability": 1.0, "probe_started_ms": 1_000,
	})
	edge, _ := json.Marshal(map[string]any{
		"sampling_reason": "WEAPON_RAISED", "event_time_lower_ms": 1_100,
		"event_time_upper_ms": 1_300, "event_time_estimate_ms": 1_200,
		"event_time_uncertainty_ms": 100, "event_timing_source": "ALIGNED_CLIENT_INTERVAL",
	})
	batch := schema.Batch{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1, Events: []schema.Event{
		{EventType: "SAMPLING_OPPORTUNITY", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, SourceEventID: "neutral", ServerSequence: 1, ServerTimeMS: 1_000, PlayerSessionID: "observer", Payload: neutral},
		{EventType: "DECISION_EDGE", Source: schema.SourceServer, SourceAuthority: schema.AuthorityClient, SourceEventID: "edge", ServerSequence: 2, ServerTimeMS: 1_200, PlayerSessionID: "observer", Payload: edge},
	}}
	got, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	observation := got[0]
	if !observation.OutcomeObserved || observation.OutcomeLowerMS != 1_100 || observation.OutcomeUpperMS != 1_300 || observation.OutcomeUncertaintyMS != 100 || observation.OutcomeTimingSource != "ALIGNED_CLIENT_INTERVAL" {
		t.Fatalf("unexpected outcome interval: %+v", observation)
	}
	if !observation.TimingEligible || !observation.ControlEligible {
		t.Fatalf("eligible neutral outcome was suppressed: %+v", observation)
	}
}

func TestBuilderSuppressesOutcomeAboveUncertaintyLimit(t *testing.T) {
	neutral, _ := json.Marshal(map[string]any{
		"classification": "NO_RELEVANT_TARGET", "control_type": "NEUTRAL_NO_RELEVANT_TARGET",
		"observer_origin_mode": "NOT_APPLICABLE", "sampling_stream": "random_opportunity",
		"sampling_policy_version": "v2", "queue_admission_probability": 1.0,
		"target_inclusion_probability": 1.0, "probe_started_ms": 1_000,
	})
	edge, _ := json.Marshal(map[string]any{
		"sampling_reason": "ADS_ENTERED", "event_time_lower_ms": 1_000,
		"event_time_upper_ms": 3_000, "event_time_estimate_ms": 2_000,
		"event_time_uncertainty_ms": 1_000, "event_timing_source": "SERVER_RECEIVE_FALLBACK",
	})
	batch := schema.Batch{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1, Events: []schema.Event{
		{EventType: "SAMPLING_OPPORTUNITY", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, SourceEventID: "neutral", ServerSequence: 1, ServerTimeMS: 1_000, PlayerSessionID: "observer", Payload: neutral},
		{EventType: "DECISION_EDGE", Source: schema.SourceServer, SourceAuthority: schema.AuthorityClient, SourceEventID: "edge", ServerSequence: 2, ServerTimeMS: 2_000, PlayerSessionID: "observer", Payload: edge},
	}}
	got, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TimingEligible || got[0].ControlEligible {
		t.Fatalf("uncertain outcome was not suppressed: %+v", got)
	}
}
