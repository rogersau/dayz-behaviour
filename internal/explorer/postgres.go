package explorer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type SQLRepository struct{ db *sql.DB }

func OpenSQLRepository(ctx context.Context, databaseURL string) (*SQLRepository, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLRepository{db: db}, nil
}

func (r *SQLRepository) Close() error { return r.db.Close() }

func (r *SQLRepository) ListSessions(ctx context.Context, query SessionQuery) ([]Session, error) {
	limit := normalLimit(query.Limit, 100, 500)
	rows, err := r.db.QueryContext(ctx, `
		SELECT ps.player_session_id,COALESCE(ps.durable_player_id,''),ps.server_id,ps.server_session_id,
		       ps.started_ms,ps.ended_ms,COUNT(ne.source_event_id)::integer
		FROM player_sessions ps
		LEFT JOIN normalized_events ne ON ne.player_session_id=ps.player_session_id
		WHERE ($1='' OR ps.server_id=$1)
		  AND ($2='' OR ps.player_session_id ILIKE '%'||$2||'%' OR COALESCE(ps.durable_player_id,'') ILIKE '%'||$2||'%' OR ps.server_id ILIKE '%'||$2||'%')
		GROUP BY ps.player_session_id
		ORDER BY ps.started_ms DESC
		LIMIT $3`, query.ServerID, query.Search, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Session, 0)
	for rows.Next() {
		var value Session
		if err := rows.Scan(&value.PlayerSessionID, &value.DurablePlayerID, &value.ServerID, &value.ServerSessionID, &value.StartedMS, &value.EndedMS, &value.EventCount); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *SQLRepository) Timeline(ctx context.Context, query TimelineQuery) (Timeline, error) {
	if strings.TrimSpace(query.PlayerSessionID) == "" {
		return Timeline{}, errors.New("player session ID is required")
	}
	var session Session
	err := r.db.QueryRowContext(ctx, `
		SELECT ps.player_session_id,COALESCE(ps.durable_player_id,''),ps.server_id,ps.server_session_id,
		       ps.started_ms,ps.ended_ms,COUNT(ne.source_event_id)::integer
		FROM player_sessions ps LEFT JOIN normalized_events ne ON ne.player_session_id=ps.player_session_id
		WHERE ps.player_session_id=$1 GROUP BY ps.player_session_id`, query.PlayerSessionID).Scan(
		&session.PlayerSessionID, &session.DurablePlayerID, &session.ServerID, &session.ServerSessionID,
		&session.StartedMS, &session.EndedMS, &session.EventCount)
	if err == sql.ErrNoRows {
		return Timeline{}, ErrNotFound
	}
	if err != nil {
		return Timeline{}, err
	}
	limit := normalLimit(query.Limit, 500, 2000)
	rows, err := r.db.QueryContext(ctx, `
		SELECT source_event_id,event_type,source,source_authority,server_time_ms,server_receive_ms,player_session_id,payload
		FROM normalized_events
		WHERE (player_session_id=$1 OR payload->>'observer_player_session_id'=$1 OR payload->>'target_player_session_id'=$1
		       OR payload->>'observer_player_id'=$1 OR payload->>'target_player_id'=$1)
		  AND ($2::bigint IS NULL OR server_time_ms >= $2)
		  AND ($3::bigint IS NULL OR server_time_ms <= $3)
		ORDER BY server_time_ms,server_sequence
		LIMIT $4`, query.PlayerSessionID, query.FromMS, query.ToMS, limit+1)
	if err != nil {
		return Timeline{}, err
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var payload []byte
		if err := rows.Scan(&entry.SourceEventID, &entry.EventType, &entry.Source, &entry.Authority, &entry.ServerTimeMS, &entry.ServerReceiveMS, &entry.PlayerSessionID, &payload); err != nil {
			return Timeline{}, err
		}
		if err := json.Unmarshal(payload, &entry.Payload); err != nil {
			return Timeline{}, fmt.Errorf("decode event %s payload: %w", entry.SourceEventID, err)
		}
		entry.RelatedSessionIDs = relatedSessions(entry.Payload, entry.PlayerSessionID)
		entry.Summary = summarize(entry)
		enrichEntry(&entry)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return Timeline{}, err
	}
	limited := len(entries) > limit
	if limited {
		entries = entries[:limit]
	}
	return Timeline{Session: session, Entries: entries, MapID: timelineMapID(entries), Limited: limited}, nil
}

func relatedSessions(payload map[string]any, primary string) []string {
	seen := map[string]bool{primary: true}
	result := make([]string, 0, 2)
	for _, key := range []string{"observer_player_session_id", "target_player_session_id", "observer_player_id", "target_player_id"} {
		value, _ := payload[key].(string)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func summarize(entry Entry) string {
	fields := entry.Payload
	switch entry.EventType {
	case "PLAYER_CONNECTED":
		return "Player connected to the server"
	case "PLAYER_DISCONNECTED":
		return "Player disconnected from the server"
	case "VISIBILITY_OBSERVATION":
		classification, _ := fields["classification"].(string)
		target, _ := fields["target_player_session_id"].(string)
		if target == "" {
			target, _ = fields["target_player_id"].(string)
		}
		return strings.TrimSpace("Visibility: " + classification + " · target " + shortID(target))
	case "WEAPON_FIRED":
		return "Weapon fired"
	case "PLAYER_HIT":
		return "Hit registered"
	case "PLAYER_KILLED":
		return "Player killed"
	case "AIM_SAMPLE":
		return "Client aim sample (untrusted telemetry)"
	default:
		return strings.ReplaceAll(strings.ToLower(entry.EventType), "_", " ")
	}
}

func shortID(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12] + "…"
}

var _ Repository = (*SQLRepository)(nil)
