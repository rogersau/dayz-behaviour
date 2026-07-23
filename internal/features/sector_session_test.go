package features_test

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/features"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestSectorExtractionDoesNotJoinAcrossServerSessions(t *testing.T) {
	edgePayload, _ := json.Marshal(map[string]any{"sampling_reason": "DELIBERATE_CAMERA_TURN", "camera_direction": []float64{0, 0, 1}})
	visibilityPayload, _ := json.Marshal(map[string]any{
		"sampling_stream": "event_enrichment", "classification": "HEAD_ORIGIN_OCCLUDED",
		"observer_origin": []float64{0, 1, 0}, "target_head": []float64{10, 1, 0},
	})
	batches := []schema.Batch{
		{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session-a", BatchSequence: 1, Events: []schema.Event{{EventType: "DECISION_EDGE", Source: schema.SourceClient, SourceAuthority: schema.AuthorityClient, ServerSequence: 1, ServerTimeMS: 1_000, PlayerSessionID: "observer", Payload: edgePayload}}},
		{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session-b", BatchSequence: 1, Events: []schema.Event{{EventType: "VISIBILITY_OBSERVATION", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, ServerSequence: 1, ServerTimeMS: 1_100, PlayerSessionID: "observer", Payload: visibilityPayload}}},
	}
	result := features.EstimateConcealedSectorForSessions([]string{"observer"}, batches)
	if result.SampleCount != 0 {
		t.Fatalf("sample count = %d, want 0", result.SampleCount)
	}
}
