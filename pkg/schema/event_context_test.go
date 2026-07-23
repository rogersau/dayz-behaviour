package schema

import (
	"encoding/json"
	"testing"
)

func TestBatchRoundTripPreservesEventContextV1(t *testing.T) {
	payload := json.RawMessage(`{
		"observer_player_id":"observer",
		"observer_player_session_id":"session:1:observer",
		"classification":"NO_RELEVANT_TARGET",
		"observer_origin_mode":"NOT_APPLICABLE",
		"sampling_stream":"random_opportunity",
		"sampling_policy_version":"dual-stream-v2-neutral-controls",
		"sampling_reason":"prospective_no_target_trigger",
		"observer_eligible_count":1,
		"observer_inclusion_probability":1,
		"target_eligible_count":0,
		"target_inclusion_probability":1,
		"risk_set_definition":"no_alive_nonself_within_radius",
		"risk_set_complete":true,
		"queue_admission_probability":1,
		"scheduler_load_state":"normal",
		"visibility_policy_version":"not_applicable",
		"probe_started_ms":1000,
		"probe_completed_ms":1000,
		"event_context":{
			"version":"v1",
			"captured_ms":1000,
			"observer":{
				"player_id":"observer",
				"player_session_id":"session:1:observer",
				"position":"100 5 200",
				"velocity":"0 0 0",
				"orientation":"90 0 0",
				"movement_heading":"0 0 0",
				"movement_speed_mps":0,
				"stance_id":0,
				"movement_state":"STOPPED",
				"alive":true,
				"unconscious":false,
				"weapon_raised":false,
				"item_in_hands_type_id":"M4A1"
			}
		}
	}`)

	batch := Batch{
		SchemaVersion:   Version1,
		ServerID:        "server",
		ServerSessionID: "session",
		BatchSequence:   1,
		ServerTimeMS:    1000,
		Events: []Event{{
			EventType:       "SAMPLING_OPPORTUNITY",
			Source:          SourceServer,
			SourceAuthority: AuthorityServer,
			ServerSequence:  1,
			ServerTimeMS:    1000,
			PlayerSessionID: "session:1:observer",
			Payload:         payload,
		}},
	}
	if err := batch.Validate(); err != nil {
		t.Fatalf("validate batch: %v", err)
	}

	encoded, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}
	var decoded Batch
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal batch: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(decoded.Events[0].Payload, &fields); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	context, ok := fields["event_context"].(map[string]any)
	if !ok {
		t.Fatal("event_context was not preserved")
	}
	if context["version"] != "v1" {
		t.Fatalf("event context version = %v, want v1", context["version"])
	}
	observer, ok := context["observer"].(map[string]any)
	if !ok || observer["player_id"] != "observer" {
		t.Fatalf("observer context was not preserved: %#v", context["observer"])
	}
}

func TestVisibilityPayloadAcceptsRawRayEvidenceV1(t *testing.T) {
	payload := json.RawMessage(`{
		"observer_player_id":"observer",
		"target_player_id":"target",
		"classification":"EXPOSED",
		"observer_origin_mode":"PLAYER_HEAD_APPROXIMATION",
		"sampling_stream":"event_enrichment",
		"sampling_policy_version":"dual-stream-v2-neutral-controls",
		"sampling_reason":"WEAPON_RAISED",
		"risk_set_definition":"alive_nonself_within_radius",
		"scheduler_load_state":"normal",
		"visibility_policy_version":"validated-first-person-head-v1",
		"observer_inclusion_probability":1,
		"target_inclusion_probability":1,
		"queue_admission_probability":1,
		"observer_eligible_count":2,
		"target_eligible_count":1,
		"probe_started_ms":1000,
		"probe_completed_ms":1001,
		"visibility_ray_evidence":{
			"version":"v1",
			"points":[{
				"point_name":"HEAD",
				"ray_origin":"100 6.7 200",
				"ray_destination":"110 6.7 200",
				"result":"CLEAR",
				"contact_present":true,
				"contact_position":"110 6.7 200",
				"contact_direction":"0 1 0",
				"contact_distance_metres":10,
				"contact_component":3,
				"contact_hierarchy_level":0,
				"contact_object_type":"SurvivorM_Mirek"
			}]
		}
	}`)
	event := Event{
		EventType:       "VISIBILITY_OBSERVATION",
		Source:          SourceServer,
		SourceAuthority: AuthorityServer,
		ServerSequence:  1,
		ServerTimeMS:    1000,
		Payload:         payload,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("validate visibility event: %v", err)
	}
}
