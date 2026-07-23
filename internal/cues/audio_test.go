package cues_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/cues"
	"github.com/rogersau/dayz-behaviour/internal/observations"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestEnrichAddsAuthoritativeGunshotCue(t *testing.T) {
	items := []observations.Observation{{
		ServerID: "server", ServerSessionID: "session", ObserverPlayerSessionID: "session:1:observer",
		TargetPlayerSessionID: "session:2:target", StartedMS: 10_000, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA",
	}}
	batches := []schema.Batch{{
		SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1,
		Events: []schema.Event{
			event("PLAYER_SNAPSHOT", "observer-position", 8_900, "session:1:observer", map[string]any{"position": []float64{0, 0, 0}}),
			event("SHOT_FIRED_SERVER", "target-shot", 9_000, "session:2:target", map[string]any{
				"source_player_id": "target", "position": []float64{0, 0, 200}, "weapon_type": "M4A1", "ammo": "Bullet_556x45", "is_suppressed": false,
			}),
		},
	}}
	if err := cues.Enrich(items, batches, cues.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if items[0].CueClass != "KNOWN" || len(items[0].CueFacts) != 1 {
		t.Fatalf("unexpected observation: %+v", items[0])
	}
	if items[0].CueFacts[0].CueType != "GUNSHOT_AUDIO_OPPORTUNITY" || !strings.Contains(items[0].CueFacts[0].Details, `"distance_metres":200`) {
		t.Fatalf("unexpected cue: %+v", items[0].CueFacts[0])
	}
}

func TestPossibleFootstepIsVisibleButDoesNotSuppress(t *testing.T) {
	items := []observations.Observation{{
		ServerID: "server", ServerSessionID: "session", ObserverPlayerSessionID: "session:1:observer",
		TargetPlayerSessionID: "session:2:target", StartedMS: 10_000, CueClass: "UNEXPLAINED_IN_CAPTURED_DATA",
	}}
	batches := []schema.Batch{{
		SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1,
		Events: []schema.Event{
			event("PLAYER_SNAPSHOT", "observer-position", 9_000, "session:1:observer", map[string]any{"position": []float64{0, 0, 0}}),
			event("MOVEMENT_AUDIO_OPPORTUNITY", "target-movement", 9_100, "session:2:target", map[string]any{
				"source_player_id": "target", "position": []float64{0, 0, 35}, "movement_speed_mps": 5.0,
				"movement_state": "SPRINT", "stance_name": "ERECT", "surface_type": "cp_concrete", "footwear_type": "CombatBoots_Black",
			}),
		},
	}}
	if err := cues.Enrich(items, batches, cues.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if items[0].CueClass != "UNEXPLAINED_IN_CAPTURED_DATA" {
		t.Fatalf("possible cue suppressed analysis: %+v", items[0])
	}
	if len(items[0].CueFacts) != 1 || !strings.Contains(items[0].CueFacts[0].Details, "POSSIBLE_AUDIO_CUE") {
		t.Fatalf("possible cue not retained: %+v", items[0].CueFacts)
	}
}

func event(eventType, id string, atMS int64, playerSessionID string, value any) schema.Event {
	payload, _ := json.Marshal(value)
	return schema.Event{
		EventType: eventType, Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer,
		SourceEventID: id, ServerSequence: uint64(atMS), ServerTimeMS: atMS,
		PlayerSessionID: playerSessionID, Payload: payload,
	}
}
