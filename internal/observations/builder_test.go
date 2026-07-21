package observations_test

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestBuilderSeparatesRandomDenominatorAndSuppressesHeadOrigin(t *testing.T) {
	batch := schema.Batch{
		SchemaVersion: 1, ServerID: "s", ServerSessionID: "session", BatchSequence: 1, ServerTimeMS: 1,
		Events: []schema.Event{
			event(1, 1_000, "VISIBILITY_OBSERVATION", "observer", map[string]any{
				"target_player_id": "target", "classification": "HEAD_ORIGIN_OCCLUDED",
				"observer_origin_mode": "PLAYER_HEAD_APPROXIMATION", "sampling_stream": "random_opportunity",
				"sampling_policy_version": "v1", "target_inclusion_probability": 0.5,
				"queue_admission_probability": 1.0, "probe_started_ms": 1_000,
			}),
			event(2, 2_000, "DECISION_EDGE", "observer", map[string]any{"sampling_reason": "WEAPON_RAISED"}),
			event(3, 3_000, "VISIBILITY_OBSERVATION", "observer", map[string]any{
				"target_player_id": "target", "classification": "ROBUSTLY_OCCLUDED",
				"observer_origin_mode": "FIRST_PERSON_EYE", "sampling_stream": "event_enrichment",
				"sampling_policy_version": "v1", "target_inclusion_probability": 0.5,
				"queue_admission_probability": 1.0, "probe_started_ms": 3_000,
			}),
		},
	}
	got, err := observations.Build([]schema.Batch{batch}, observations.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 random opportunity", len(got))
	}
	if !got[0].OutcomeObserved || got[0].StrongHiddenEligible {
		t.Fatalf("observation = %+v", got[0])
	}
}

func event(sequence uint64, at int64, kind, player string, payload any) schema.Event {
	data, _ := json.Marshal(payload)
	return schema.Event{
		EventType: kind, Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer,
		SourceEventID: "event-" + string(rune('0'+sequence)), ServerSequence: sequence,
		ServerTimeMS: at, PlayerSessionID: player, Payload: data,
	}
}
