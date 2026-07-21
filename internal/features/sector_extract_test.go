package features

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestExtractsConcealedSectorFromEdgeAndEnrichment(t *testing.T) {
	edgePayload, _ := json.Marshal(map[string]any{"sampling_reason": "DELIBERATE_CAMERA_TURN", "camera_direction": []float64{1, 0, 0}})
	probePayload, _ := json.Marshal(map[string]any{"sampling_stream": "event_enrichment", "classification": "HEAD_ORIGIN_OCCLUDED", "observer_origin": []float64{0, 0, 0}, "target_head": []float64{10, 1, 0}})
	batch := schema.Batch{Events: []schema.Event{{EventType: "DECISION_EDGE", PlayerSessionID: "p", ServerSequence: 1, ServerTimeMS: 100, Payload: edgePayload}, {EventType: "VISIBILITY_OBSERVATION", PlayerSessionID: "p", ServerSequence: 2, ServerTimeMS: 200, Payload: probePayload}}}
	result := EstimateConcealedSectorForSessions([]string{"p"}, []schema.Batch{batch})
	if result.SampleCount != 1 || result.ObservedConcentration < .99 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
