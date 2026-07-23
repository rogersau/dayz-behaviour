package postgres

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizePayloadPreservesEventContextAndRayEvidence(t *testing.T) {
	raw := json.RawMessage(`{
		"event_context":{
			"version":"v1",
			"captured_ms":2500,
			"observer":{"player_id":"observer","position":"1 2 3"},
			"target":{"player_id":"target","position":"4 5 6"}
		},
		"visibility_ray_evidence":{
			"version":"v1",
			"points":[{
				"point_name":"TORSO",
				"ray_origin":"1 2 3",
				"ray_destination":"4 5 6",
				"result":"HARD_BLOCKED",
				"contact_present":true,
				"contact_position":"2 3 4",
				"contact_component":7,
				"contact_object_type":"Land_House_1W01",
				"blocker_type":"Land_House_1W01"
			}]
		}
	}`)

	normalized, fields, err := normalizePayload(raw)
	if err != nil {
		t.Fatalf("normalize payload: %v", err)
	}
	if _, ok := fields["event_context"].(map[string]any); !ok {
		t.Fatalf("event_context missing from normalized fields: %#v", fields)
	}
	if _, ok := fields["visibility_ray_evidence"].(map[string]any); !ok {
		t.Fatalf("visibility_ray_evidence missing from normalized fields: %#v", fields)
	}

	var originalFields map[string]any
	if err := json.Unmarshal(raw, &originalFields); err != nil {
		t.Fatalf("unmarshal original payload: %v", err)
	}
	var normalizedFields map[string]any
	if err := json.Unmarshal(normalized, &normalizedFields); err != nil {
		t.Fatalf("unmarshal normalized payload: %v", err)
	}
	if !reflect.DeepEqual(originalFields, normalizedFields) {
		t.Fatalf("nested evidence changed during normalization\noriginal: %#v\nnormalized: %#v", originalFields, normalizedFields)
	}
}
