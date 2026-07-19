package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	Version1            = 1
	MaxEventsPerBatch   = 1_000
	MaxEventTypeLength  = 64
	MaxIdentifierLength = 128
	MaxPayloadBytes     = 32 * 1024
)

var (
	ErrUnsupportedVersion = errors.New("unsupported schema version")
	ErrInvalidBatch       = errors.New("invalid telemetry batch")
)

// Batch is the durable transport envelope sent by a DayZ server collector.
// Client-supplied telemetry remains untrusted even after the server wraps it.
type Batch struct {
	SchemaVersion   int     `json:"schema_version"`
	ServerID        string  `json:"server_id"`
	ServerSessionID string  `json:"server_session_id"`
	BatchSequence   uint64  `json:"batch_sequence"`
	ServerTimeMS    int64   `json:"server_time_ms"`
	Events          []Event `json:"events"`
}

// Event is intentionally flexible for Milestones 0-1. Event-specific payloads
// are versioned independently and normalised after durable ingestion.
type Event struct {
	EventType             string          `json:"event_type"`
	Source                EventSource     `json:"source"`
	ServerSequence        uint64          `json:"server_sequence"`
	ServerTimeMS          int64           `json:"server_time_ms"`
	PlayerSessionID       string          `json:"player_session_id,omitempty"`
	PlayerID              string          `json:"player_id,omitempty"`
	ClientSequence        *uint64         `json:"client_sequence,omitempty"`
	ClientMonotonicTimeMS *int64          `json:"client_monotonic_time_ms,omitempty"`
	Payload               json.RawMessage `json:"payload,omitempty"`
}

type EventSource string

const (
	SourceServer EventSource = "server"
	SourceClient EventSource = "client_untrusted"
)

func (b Batch) Validate() error {
	if b.SchemaVersion != Version1 {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, b.SchemaVersion, Version1)
	}
	if err := validateIdentifier("server_id", b.ServerID, true); err != nil {
		return err
	}
	if err := validateIdentifier("server_session_id", b.ServerSessionID, true); err != nil {
		return err
	}
	if b.BatchSequence == 0 {
		return fmt.Errorf("%w: batch_sequence must be greater than zero", ErrInvalidBatch)
	}
	if b.ServerTimeMS < 0 {
		return fmt.Errorf("%w: server_time_ms must not be negative", ErrInvalidBatch)
	}
	if len(b.Events) == 0 || len(b.Events) > MaxEventsPerBatch {
		return fmt.Errorf("%w: events must contain 1-%d entries", ErrInvalidBatch, MaxEventsPerBatch)
	}
	for index, event := range b.Events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("%w: events[%d]: %v", ErrInvalidBatch, index, err)
		}
	}
	return nil
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.EventType) == "" || len(e.EventType) > MaxEventTypeLength {
		return fmt.Errorf("event_type must contain 1-%d characters", MaxEventTypeLength)
	}
	if e.Source != SourceServer && e.Source != SourceClient {
		return fmt.Errorf("source must be %q or %q", SourceServer, SourceClient)
	}
	if e.ServerSequence == 0 {
		return errors.New("server_sequence must be greater than zero")
	}
	if e.ServerTimeMS < 0 {
		return errors.New("server_time_ms must not be negative")
	}
	if err := validateIdentifier("player_session_id", e.PlayerSessionID, false); err != nil {
		return err
	}
	if err := validateIdentifier("player_id", e.PlayerID, false); err != nil {
		return err
	}
	if e.Source == SourceClient && e.ClientSequence == nil {
		return errors.New("client_untrusted events require client_sequence")
	}
	if len(e.Payload) > MaxPayloadBytes {
		return fmt.Errorf("payload exceeds %d bytes", MaxPayloadBytes)
	}
	if len(e.Payload) > 0 && !json.Valid(e.Payload) {
		return errors.New("payload must be valid JSON")
	}
	return nil
}

func validateIdentifier(name, value string, required bool) error {
	trimmed := strings.TrimSpace(value)
	if required && trimmed == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidBatch, name)
	}
	if len(value) > MaxIdentifierLength {
		return fmt.Errorf("%w: %s exceeds %d characters", ErrInvalidBatch, name, MaxIdentifierLength)
	}
	return nil
}
