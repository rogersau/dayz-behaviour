package observations_test

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestBuilderSeparatesNeutralAndVisibleControls(t *testing.T) {
	neutral, _ := json.Marshal(map[string]any{
		"classification": "NO_RELEVANT_TARGET", "control_type": "NEUTRAL_NO_RELEVANT_TARGET",
		"observer_origin_mode": "NOT_APPLICABLE", "sampling_stream": "random_opportunity",
		"sampling_policy_version": "dual-stream-v2-neutral-controls", "queue_admission_probability": 1.0,
		"target_inclusion_probability": 1.0, "probe_started_ms": 1_000,
	})
	visible, _ := json.Marshal(map[string]any{
		"target_player_id": "target", "classification": "EXPOSED",
		"observer_origin_mode": "PLAYER_HEAD_APPROXIMATION", "sampling_stream": "random_opportunity",
		"sampling_policy_version": "dual-stream-v2-neutral-controls", "queue_admission_probability": 1.0,
		"target_inclusion_probability": 1.0, "probe_started_ms": 7_000,
	})
	batch := schema.Batch{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1, Events: []schema.Event{
		{EventType: "SAMPLING_OPPORTUNITY", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, SourceEventID: "neutral", ServerSequence: 1, ServerTimeMS: 1_000, PlayerSessionID: "observer", Payload: neutral},
		{EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, SourceEventID: "visible", ServerSequence: 2, ServerTimeMS: 7_000, PlayerSessionID: "observer", Payload: visible},
	}}
	got, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if !got[0].ControlEligible || got[0].PositiveControlEligible || got[0].ControlKind != "NEUTRAL_NO_RELEVANT_TARGET" {
		t.Fatalf("neutral observation = %+v", got[0])
	}
	if got[1].ControlEligible || !got[1].PositiveControlEligible || got[1].ControlKind != "VISIBLE_TARGET" {
		t.Fatalf("visible observation = %+v", got[1])
	}
}
