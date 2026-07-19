package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestBatchValidate(t *testing.T) {
	clientSequence := uint64(2)
	clientTime := int64(90)
	batch := schema.Batch{
		SchemaVersion:   schema.Version1,
		ServerID:        "server-a",
		ServerSessionID: "session-a",
		BatchSequence:   1,
		ServerTimeMS:    1,
		Events: []schema.Event{{
			EventType:             "CAMERA_SAMPLE",
			Source:                schema.SourceClient,
			ServerSequence:        1,
			ServerTimeMS:          100,
			PlayerSessionID:       "player-session",
			PlayerID:              "player-id",
			ClientSequence:        &clientSequence,
			ClientMonotonicTimeMS: &clientTime,
			Payload:               json.RawMessage(`{"camera_direction":[0,0,1]}`),
		}},
	}
	if err := batch.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestClientEventRequiresSequence(t *testing.T) {
	event := schema.Event{
		EventType:      "CAMERA_SAMPLE",
		Source:         schema.SourceClient,
		ServerSequence: 1,
		ServerTimeMS:   1,
	}
	if err := event.Validate(); err == nil {
		t.Fatal("Validate returned nil, want error")
	}
}
