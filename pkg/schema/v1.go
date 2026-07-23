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
	SchemaVersion     int     `json:"schema_version"`
	ServerID          string  `json:"server_id"`
	ServerSessionID   string  `json:"server_session_id"`
	BatchSequence     uint64  `json:"batch_sequence"`
	ServerTimeMS      int64   `json:"server_time_ms"`
	CollectorVersion  string  `json:"collector_version,omitempty"`
	DayZBuild         string  `json:"dayz_build,omitempty"`
	ConfigurationHash string  `json:"configuration_hash,omitempty"`
	Events            []Event `json:"events"`
}

// Event is intentionally flexible for Milestones 0-1. Event-specific payloads
// are versioned independently and normalised after durable ingestion.
type Event struct {
	EventType             string          `json:"event_type"`
	Source                EventSource     `json:"source"`
	SourceAuthority       SourceAuthority `json:"source_authority,omitempty"`
	SourceComponent       string          `json:"source_component,omitempty"`
	SourceEventID         string          `json:"source_event_id,omitempty"`
	SourceSchemaVersion   int             `json:"source_schema_version,omitempty"`
	CollectorVersion      string          `json:"collector_version,omitempty"`
	ServerSequence        uint64          `json:"server_sequence"`
	ServerTimeMS          int64           `json:"server_time_ms"`
	ServerReceiveMS       int64           `json:"server_receive_ms,omitempty"`
	PlayerSessionID       string          `json:"player_session_id,omitempty"`
	PlayerID              string          `json:"player_id,omitempty"`
	ClientSequence        *uint64         `json:"client_sequence,omitempty"`
	ClientMonotonicTimeMS *int64          `json:"client_monotonic_time_ms,omitempty"`
	Payload               json.RawMessage `json:"payload,omitempty"`
}

type EventSource string
type SourceAuthority string

const (
	SourceServer    EventSource     = "server"
	SourceClient    EventSource     = "client_untrusted"
	AuthorityServer SourceAuthority = "A"
	AuthorityClient SourceAuthority = "B"
	AuthorityHealth SourceAuthority = "C"
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
	if e.SourceAuthority != "" {
		if e.SourceAuthority != AuthorityServer && e.SourceAuthority != AuthorityClient && e.SourceAuthority != AuthorityHealth {
			return fmt.Errorf("source_authority must be %q, %q or %q", AuthorityServer, AuthorityClient, AuthorityHealth)
		}
		if e.Source == SourceClient && e.SourceAuthority != AuthorityClient && e.SourceAuthority != AuthorityHealth {
			return errors.New("client_untrusted events cannot claim server authority")
		}
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
	if e.EventType == "VISIBILITY_OBSERVATION" {
		if e.Source != SourceServer || e.SourceAuthority != AuthorityServer {
			return errors.New("visibility observations require Tier A server authority")
		}
		if err := validateVisibilityPayload(e.Payload); err != nil {
			return err
		}
	}
	return nil
}

func validateVisibilityPayload(raw json.RawMessage) error {
	var fields map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &fields) != nil {
		return errors.New("visibility observation requires an object payload")
	}
	requireString := func(key string) error {
		value, ok := fields[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("visibility payload %s is required", key)
		}
		return nil
	}
	for _, key := range []string{"observer_player_id", "target_player_id", "classification", "observer_origin_mode", "sampling_stream", "sampling_policy_version", "sampling_reason", "risk_set_definition", "scheduler_load_state", "visibility_policy_version"} {
		if err := requireString(key); err != nil {
			return err
		}
	}
	stream, _ := fields["sampling_stream"].(string)
	if stream != "random_opportunity" && stream != "event_enrichment" {
		return errors.New("visibility payload sampling_stream is invalid")
	}
	positive := func(key string) bool { value, ok := fields[key].(float64); return ok && value > 0 && value <= 1 }
	if !positive("observer_inclusion_probability") || !positive("target_inclusion_probability") || !positive("queue_admission_probability") {
		return errors.New("visibility inclusion and queue-admission probabilities must be in (0,1]")
	}
	for _, key := range []string{"observer_eligible_count", "target_eligible_count", "probe_started_ms", "probe_completed_ms"} {
		value, ok := fields[key].(float64)
		if !ok || value < 0 {
			return fmt.Errorf("visibility payload %s must be non-negative", key)
		}
	}
	classification, _ := fields["classification"].(string)
	origin, _ := fields["observer_origin_mode"].(string)
	if classification == "ROBUSTLY_OCCLUDED" {
		validationID, _ := fields["visibility_validation_id"].(string)
		duration, _ := fields["occlusion_duration_ms"].(float64)
		if !validatedFirstPersonOrigin(origin) || strings.TrimSpace(validationID) == "" || duration <= 0 {
			return errors.New("robust occlusion requires a validated first-person origin and positive duration")
		}
	}
	return nil
}

func validatedFirstPersonOrigin(origin string) bool {
	return origin == "VALIDATED_FIRST_PERSON_HEAD" || origin == "FIRST_PERSON_EYE"
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
