package explorer

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
)

type SessionQuery struct {
	ServerID string
	Search   string
	Limit    int
}

type TimelineQuery struct {
	PlayerSessionID string
	FromMS          *int64
	ToMS            *int64
	Limit           int
}

type Session struct {
	PlayerSessionID string `json:"player_session_id"`
	DurablePlayerID string `json:"durable_player_id,omitempty"`
	ServerID        string `json:"server_id"`
	ServerSessionID string `json:"server_session_id"`
	StartedMS       int64  `json:"started_ms"`
	EndedMS         *int64 `json:"ended_ms,omitempty"`
	EventCount      int    `json:"event_count"`
}

type Entry struct {
	SourceEventID     string         `json:"source_event_id"`
	EventType         string         `json:"event_type"`
	Source            string         `json:"source"`
	Authority         string         `json:"authority"`
	ServerTimeMS      int64          `json:"server_time_ms"`
	ServerReceiveMS   int64          `json:"server_receive_ms"`
	PlayerSessionID   string         `json:"player_session_id"`
	RelatedSessionIDs []string       `json:"related_session_ids,omitempty"`
	Summary           string         `json:"summary"`
	Payload           map[string]any `json:"payload"`
	Location          *Location      `json:"location,omitempty"`
}

type Timeline struct {
	Session Session `json:"session"`
	Entries []Entry `json:"entries"`
	MapID   string  `json:"map_id,omitempty"`
	Limited bool    `json:"limited"`
}

type Location struct {
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	Z                float64 `json:"z"`
	AccuracyMetres   float64 `json:"accuracy_metres"`
	SourceField      string  `json:"source_field"`
	SubjectSessionID string  `json:"subject_session_id"`
}

type Repository interface {
	ListSessions(context.Context, SessionQuery) ([]Session, error)
	Timeline(context.Context, TimelineQuery) (Timeline, error)
}

type MemoryRepository struct {
	Sessions []Session
	Entries  map[string][]Entry
}

func (r *MemoryRepository) ListSessions(_ context.Context, query SessionQuery) ([]Session, error) {
	limit := normalLimit(query.Limit, 100, 500)
	result := make([]Session, 0, len(r.Sessions))
	for _, session := range r.Sessions {
		if query.ServerID != "" && query.ServerID != session.ServerID {
			continue
		}
		if query.Search != "" && !containsFold(session.PlayerSessionID+" "+session.DurablePlayerID+" "+session.ServerID, query.Search) {
			continue
		}
		result = append(result, session)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartedMS > result[j].StartedMS })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *MemoryRepository) Timeline(_ context.Context, query TimelineQuery) (Timeline, error) {
	if query.PlayerSessionID == "" {
		return Timeline{}, errors.New("player session ID is required")
	}
	var session Session
	found := false
	for _, candidate := range r.Sessions {
		if candidate.PlayerSessionID == query.PlayerSessionID {
			session, found = candidate, true
			break
		}
	}
	if !found {
		return Timeline{}, ErrNotFound
	}
	limit := normalLimit(query.Limit, 500, 2000)
	entries := make([]Entry, 0, len(r.Entries[query.PlayerSessionID]))
	for _, entry := range r.Entries[query.PlayerSessionID] {
		if query.FromMS != nil && entry.ServerTimeMS < *query.FromMS || query.ToMS != nil && entry.ServerTimeMS > *query.ToMS {
			continue
		}
		enrichEntry(&entry)
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ServerTimeMS < entries[j].ServerTimeMS })
	limited := len(entries) > limit
	if limited {
		entries = entries[:limit]
	}
	return Timeline{Session: session, Entries: entries, MapID: timelineMapID(entries), Limited: limited}, nil
}

var ErrNotFound = errors.New("player session not found")

func normalLimit(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func containsFold(value, search string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(search))
}

func enrichEntry(entry *Entry) {
	if entry.Location != nil {
		return
	}
	for _, field := range []struct {
		name     string
		accuracy float64
	}{
		{"position", 1},
		{"observer_origin", 1},
		{"model_position", 2},
		{"camera_position", 5},
		{"client_observed_player_position", 5},
	} {
		x, y, z, ok := vector(entry.Payload[field.name])
		if !ok || (x == 0 && y == 0 && z == 0) {
			continue
		}
		entry.Location = &Location{X: x, Y: y, Z: z, AccuracyMetres: field.accuracy, SourceField: field.name, SubjectSessionID: entry.PlayerSessionID}
		return
	}
	cell, _ := entry.Payload["area_cell"].(string)
	parts := strings.Split(cell, ":")
	if len(parts) != 2 {
		return
	}
	x, xErr := strconv.ParseFloat(parts[0], 64)
	z, zErr := strconv.ParseFloat(parts[1], 64)
	if xErr == nil && zErr == nil {
		entry.Location = &Location{X: x*100 + 50, Z: z*100 + 50, AccuracyMetres: 71, SourceField: "area_cell", SubjectSessionID: entry.PlayerSessionID}
	}
}

func vector(value any) (float64, float64, float64, bool) {
	if items, ok := value.([]any); ok && len(items) >= 3 {
		x, xOK := number(items[0])
		y, yOK := number(items[1])
		z, zOK := number(items[2])
		return x, y, z, xOK && yOK && zOK
	}
	if text, ok := value.(string); ok {
		parts := strings.Fields(strings.NewReplacer(",", " ", "[", " ", "]", " ").Replace(text))
		if len(parts) >= 3 {
			x, xErr := strconv.ParseFloat(parts[0], 64)
			y, yErr := strconv.ParseFloat(parts[1], 64)
			z, zErr := strconv.ParseFloat(parts[2], 64)
			return x, y, z, xErr == nil && yErr == nil && zErr == nil
		}
	}
	return 0, 0, 0, false
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func timelineMapID(entries []Entry) string {
	for _, entry := range entries {
		if value, _ := entry.Payload["map_id"].(string); strings.TrimSpace(value) != "" {
			return strings.ToLower(strings.TrimSpace(value))
		}
	}
	return ""
}
