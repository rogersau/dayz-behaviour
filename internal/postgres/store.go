package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rogersau/dayz-behaviour/internal/identity"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db *sql.DB
}

type BatchRef struct {
	ServerID        string
	ServerSessionID string
	BatchSequence   uint64
}

type RetentionCutoffs struct {
	Raw        *time.Time
	Normalized *time.Time
	Review     *time.Time
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DeletePlayer removes subject-level normalized and review data after the raw
// store has been scrubbed. Aggregate calibration records may be retained only
// when they no longer reference a subject. The audit record retains the direct
// durable identity so deletion actions can be cross-referenced operationally.
func (s *Store) DeletePlayer(ctx context.Context, durablePlayerID, actor, reason string, affectedBatches []BatchRef) (map[string]int64, error) {
	if strings.TrimSpace(durablePlayerID) == "" || strings.TrimSpace(actor) == "" || strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("durable player identity, actor and reason are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	directPlayerID := pseudonymousDurableID(durablePlayerID)
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE deletion_sessions ON COMMIT DROP AS SELECT player_session_id FROM player_sessions WHERE durable_player_id=$1`, directPlayerID); err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	deleteCount := func(name, query string, args ...any) error {
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("delete %s: %w", name, err)
		}
		counts[name], _ = result.RowsAffected()
		return nil
	}

	queries := []struct{ name, sql string }{
		{"review_dispositions", `DELETE FROM review_dispositions WHERE review_case_id IN (SELECT rc.review_case_id FROM review_cases rc JOIN candidate_rankings cr USING(candidate_ranking_id) WHERE cr.durable_player_id=$1)`},
		{"review_cases", `DELETE FROM review_cases WHERE candidate_ranking_id IN (SELECT candidate_ranking_id FROM candidate_rankings WHERE durable_player_id=$1)`},
		{"candidate_rankings", `DELETE FROM candidate_rankings WHERE durable_player_id=$1`},
		{"feature_results", `DELETE FROM feature_results WHERE player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR player_session_id=$1`},
		{"matched_control_sets", `DELETE FROM matched_control_sets WHERE observation_id IN (SELECT ao.observation_id FROM analysis_observations ao JOIN decision_windows dw USING(decision_window_id) JOIN sampling_opportunities so USING(opportunity_id) WHERE so.observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR so.target_player_session_id IN (SELECT player_session_id FROM deletion_sessions))`},
		{"analysis_observations", `DELETE FROM analysis_observations WHERE decision_window_id IN (SELECT dw.decision_window_id FROM decision_windows dw JOIN sampling_opportunities so USING(opportunity_id) WHERE so.observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR so.target_player_session_id IN (SELECT player_session_id FROM deletion_sessions))`},
		{"decision_windows", `DELETE FROM decision_windows WHERE opportunity_id IN (SELECT opportunity_id FROM sampling_opportunities WHERE observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR target_player_session_id IN (SELECT player_session_id FROM deletion_sessions))`},
		{"cue_facts", `DELETE FROM cue_facts WHERE observer_target_episode_id IN (SELECT observer_target_episode_id FROM observer_target_episodes WHERE observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR target_player_session_id IN (SELECT player_session_id FROM deletion_sessions))`},
		{"observer_target_episodes", `DELETE FROM observer_target_episodes WHERE observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR target_player_session_id IN (SELECT player_session_id FROM deletion_sessions))`},
		{"visibility_probe_runs", `DELETE FROM visibility_probe_runs WHERE observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR target_player_session_id IN (SELECT player_session_id FROM deletion_sessions)`},
		{"sampling_opportunities", `DELETE FROM sampling_opportunities WHERE observer_player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR target_player_session_id IN (SELECT player_session_id FROM deletion_sessions)`},
		{"normalized_events", `DELETE FROM normalized_events WHERE player_session_id IN (SELECT player_session_id FROM deletion_sessions) OR payload->>'observer_player_session_id' IN (SELECT player_session_id FROM deletion_sessions) OR payload->>'target_player_session_id' IN (SELECT player_session_id FROM deletion_sessions) OR payload->>'observer_player_id'=$1 OR payload->>'target_player_id'=$1 OR payload->>'source_player_id'=$1 OR payload->>'player_id'=$1`},
		{"player_sessions", `DELETE FROM player_sessions WHERE player_session_id IN (SELECT player_session_id FROM deletion_sessions)`},
	}
	for _, query := range queries {
		var err error
		if strings.Contains(query.sql, "$1") {
			err = deleteCount(query.name, query.sql, directPlayerID)
		} else {
			err = deleteCount(query.name, query.sql)
		}
		if err != nil {
			return nil, err
		}
	}
	for _, batch := range affectedBatches {
		if err := deleteCount("raw_batch_metadata", `DELETE FROM raw_batches WHERE server_id=$1 AND server_session_id=$2 AND batch_sequence=$3`, batch.ServerID, batch.ServerSessionID, batch.BatchSequence); err != nil {
			return nil, err
		}
	}
	encodedCounts, err := json.Marshal(counts)
	if err != nil {
		return nil, err
	}
	auditID := "privacy_" + digest(fmt.Sprintf("%s:%s:%s:%v", directPlayerID, actor, reason, affectedBatches))
	if _, err := tx.ExecContext(ctx, `INSERT INTO privacy_audit_log(privacy_audit_id,action,subject_pseudonym,actor,reason,affected_counts) VALUES($1,'DELETE_PLAYER',$2,$3,$4,$5) ON CONFLICT DO NOTHING`, auditID, directPlayerID, actor, reason, encodedCounts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return counts, nil
}

// ApplyRetention enforces independently configured database cutoffs. When
// execute is false the transaction is rolled back after producing exact counts.
func (s *Store) ApplyRetention(ctx context.Context, cutoffs RetentionCutoffs, execute bool) (map[string]int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	counts := map[string]int64{}
	run := func(name, query string, arg time.Time) error {
		result, err := tx.ExecContext(ctx, query, arg)
		if err != nil {
			return fmt.Errorf("retention %s: %w", name, err)
		}
		counts[name], _ = result.RowsAffected()
		return nil
	}
	if cutoffs.Review != nil {
		if err := run("review_dispositions", `DELETE FROM review_dispositions WHERE review_case_id IN (SELECT rc.review_case_id FROM review_cases rc JOIN candidate_rankings cr USING(candidate_ranking_id) WHERE cr.created_at < $1)`, *cutoffs.Review); err != nil {
			return nil, err
		}
		if err := run("review_cases", `DELETE FROM review_cases WHERE candidate_ranking_id IN (SELECT candidate_ranking_id FROM candidate_rankings WHERE created_at < $1)`, *cutoffs.Review); err != nil {
			return nil, err
		}
		if err := run("candidate_rankings", `DELETE FROM candidate_rankings WHERE created_at < $1`, *cutoffs.Review); err != nil {
			return nil, err
		}
	}
	if cutoffs.Normalized != nil {
		if err := run("matched_control_sets", `DELETE FROM matched_control_sets WHERE observation_id IN (SELECT ao.observation_id FROM analysis_observations ao JOIN decision_windows dw USING(decision_window_id) JOIN sampling_opportunities so USING(opportunity_id) JOIN normalized_events ne ON ne.source_event_id=so.source_event_id WHERE ne.normalized_at < $1)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("analysis_observations", `DELETE FROM analysis_observations WHERE decision_window_id IN (SELECT dw.decision_window_id FROM decision_windows dw JOIN sampling_opportunities so USING(opportunity_id) JOIN normalized_events ne ON ne.source_event_id=so.source_event_id WHERE ne.normalized_at < $1)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("decision_windows", `DELETE FROM decision_windows WHERE opportunity_id IN (SELECT so.opportunity_id FROM sampling_opportunities so JOIN normalized_events ne ON ne.source_event_id=so.source_event_id WHERE ne.normalized_at < $1)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("cue_facts", `DELETE FROM cue_facts WHERE source_event_id IN (SELECT source_event_id FROM normalized_events WHERE normalized_at < $1)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("visibility_probe_runs", `DELETE FROM visibility_probe_runs WHERE source_event_id IN (SELECT source_event_id FROM normalized_events WHERE normalized_at < $1)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("sampling_opportunities", `DELETE FROM sampling_opportunities WHERE source_event_id IN (SELECT source_event_id FROM normalized_events WHERE normalized_at < $1)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("normalized_events", `DELETE FROM normalized_events WHERE normalized_at < $1 AND source_event_id NOT IN (SELECT source_event_id FROM cue_facts) AND source_event_id NOT IN (SELECT source_event_id FROM visibility_probe_runs) AND source_event_id NOT IN (SELECT source_event_id FROM sampling_opportunities)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("player_sessions", `DELETE FROM player_sessions ps WHERE $1::timestamptz IS NOT NULL AND ended_ms IS NOT NULL AND NOT EXISTS (SELECT 1 FROM normalized_events ne WHERE ne.player_session_id=ps.player_session_id)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("observer_target_episodes", `DELETE FROM observer_target_episodes ote WHERE $1::timestamptz IS NOT NULL AND NOT EXISTS (SELECT 1 FROM decision_windows dw WHERE dw.observer_target_episode_id=ote.observer_target_episode_id) AND NOT EXISTS (SELECT 1 FROM cue_facts cf WHERE cf.observer_target_episode_id=ote.observer_target_episode_id)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
		if err := run("encounters", `DELETE FROM encounters e WHERE $1::timestamptz IS NOT NULL AND NOT EXISTS (SELECT 1 FROM observer_target_episodes ote WHERE ote.encounter_id=e.encounter_id)`, *cutoffs.Normalized); err != nil {
			return nil, err
		}
	}
	if cutoffs.Raw != nil {
		if err := run("raw_batch_metadata", `DELETE FROM raw_batches WHERE ingested_at < $1`, *cutoffs.Raw); err != nil {
			return nil, err
		}
	}
	if !execute {
		return counts, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return counts, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now(), checksum_sha256 text NOT NULL DEFAULT '')`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum_sha256 text NOT NULL DEFAULT ''`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		checksum := hex.EncodeToString(sum[:])
		var storedChecksum string
		err = s.db.QueryRowContext(ctx, `SELECT checksum_sha256 FROM schema_migrations WHERE version=$1`, entry.Name()).Scan(&storedChecksum)
		switch {
		case err == nil:
			if storedChecksum == "" {
				if _, err := s.db.ExecContext(ctx, `UPDATE schema_migrations SET checksum_sha256=$2 WHERE version=$1`, entry.Name(), checksum); err != nil {
					return err
				}
				continue
			}
			if storedChecksum != checksum {
				return fmt.Errorf("migration checksum mismatch for %s", entry.Name())
			}
			continue
		case err != sql.ErrNoRows:
			return err
		}

		if _, err := s.db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations(version,checksum_sha256) VALUES($1,$2)`, entry.Name(), checksum); err != nil {
			return err
		}
	}
	return s.ensurePseudonymPolicy(ctx)
}

func (s *Store) ensurePseudonymPolicy(ctx context.Context) error {
	policy, err := identity.CurrentPolicy()
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pseudonym_policy_state(singleton,policy_version,key_id) VALUES(true,$1,$2) ON CONFLICT(singleton) DO NOTHING`, policy.Version, policy.KeyID); err != nil {
		return err
	}
	var storedVersion, storedKeyID string
	if err := s.db.QueryRowContext(ctx, `SELECT policy_version,key_id FROM pseudonym_policy_state WHERE singleton=true`).Scan(&storedVersion, &storedKeyID); err != nil {
		return err
	}
	if storedVersion != policy.Version || storedKeyID != policy.KeyID {
		return fmt.Errorf("pseudonym policy mismatch: database uses %s/%s, process uses %s/%s", storedVersion, storedKeyID, policy.Version, policy.KeyID)
	}
	return nil
}

func (s *Store) Accept(ctx context.Context, batch schema.Batch) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	first, last := sequenceBounds(batch.Events)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO raw_batches (
			server_id, server_session_id, batch_sequence, schema_version, server_time_ms,
			collector_version, dayz_build, configuration_hash, content_sha256, byte_length,
			first_server_sequence, last_server_sequence
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (server_id, server_session_id, batch_sequence) DO NOTHING`,
		batch.ServerID, batch.ServerSessionID, batch.BatchSequence, batch.SchemaVersion, batch.ServerTimeMS,
		batch.CollectorVersion, batch.DayZBuild, batch.ConfigurationHash, hex.EncodeToString(sum[:]), len(data), first, last,
	)
	if err != nil {
		return err
	}
	var storedHash string
	if err := tx.QueryRowContext(ctx, `SELECT content_sha256 FROM raw_batches WHERE server_id=$1 AND server_session_id=$2 AND batch_sequence=$3`, batch.ServerID, batch.ServerSessionID, batch.BatchSequence).Scan(&storedHash); err != nil {
		return err
	}
	if storedHash != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("batch idempotency conflict for %s/%s/%d", batch.ServerID, batch.ServerSessionID, batch.BatchSequence)
	}

	for _, event := range batch.Events {
		sourceEventID := event.SourceEventID
		if sourceEventID == "" {
			sourceEventID = fmt.Sprintf("%s:%d", batch.ServerSessionID, event.ServerSequence)
		}
		authority := event.SourceAuthority
		if authority == "" {
			authority = schema.AuthorityServer
			if event.Source == schema.SourceClient {
				authority = schema.AuthorityClient
			}
		}
		payload, fields, err := normalizePayload(event.Payload)
		if err != nil {
			return fmt.Errorf("normalize payload %s: %w", sourceEventID, err)
		}
		playerSessionID := pseudonymousSessionID(event.PlayerSessionID)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO normalized_events (
				source_event_id, server_id, server_session_id, batch_sequence, event_type,
				source, source_authority, source_component, source_schema_version, collector_version,
				server_sequence, server_time_ms, server_receive_ms, player_session_id,
				client_sequence, client_monotonic_time_ms, payload, normalized_event_schema_version
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,'v3-direct-identities')
			ON CONFLICT (source_event_id) DO NOTHING`,
			sourceEventID, batch.ServerID, batch.ServerSessionID, batch.BatchSequence, event.EventType,
			event.Source, authority, event.SourceComponent, event.SourceSchemaVersion, event.CollectorVersion,
			event.ServerSequence, event.ServerTimeMS, event.ServerReceiveMS, playerSessionID,
			event.ClientSequence, event.ClientMonotonicTimeMS, payload,
		)
		if err != nil {
			return err
		}
		if playerSessionID != "" {
			endedMS := any(nil)
			if event.EventType == "PLAYER_DISCONNECTED" {
				endedMS = event.ServerTimeMS
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO player_sessions(player_session_id,server_id,server_session_id,durable_player_id,started_ms,ended_ms,reconnect_semantics_version)
				VALUES($1,$2,$3,$4,$5,$6,'lifecycle-v1')
				ON CONFLICT(player_session_id) DO UPDATE SET
					started_ms=LEAST(player_sessions.started_ms,EXCLUDED.started_ms),
					ended_ms=COALESCE(EXCLUDED.ended_ms,player_sessions.ended_ms)`,
				playerSessionID, batch.ServerID, batch.ServerSessionID, pseudonymousDurableID(rawDurableID(event.PlayerSessionID)), event.ServerTimeMS, endedMS)
			if err != nil {
				return err
			}
		}
		switch event.EventType {
		case "VISIBILITY_OBSERVATION":
			if err := insertSamplingOpportunity(ctx, tx, sourceEventID, playerSessionID, fields); err != nil {
				return err
			}
			if err := insertVisibilityProbe(ctx, tx, sourceEventID, playerSessionID, fields); err != nil {
				return err
			}
		case "SAMPLING_OPPORTUNITY", "SAMPLING_OPPORTUNITY_DROPPED":
			if err := insertSamplingOpportunity(ctx, tx, sourceEventID, playerSessionID, fields); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func normalizePayload(raw json.RawMessage) (json.RawMessage, map[string]any, error) {
	fields := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, nil, err
		}
	}
	for _, key := range []string{"observer_player_session_id", "target_player_session_id"} {
		if value := stringField(fields, key); value != "" {
			fields[key] = pseudonymousSessionID(value)
		}
	}
	for _, key := range []string{"observer_player_id", "target_player_id", "source_player_id", "player_id"} {
		if value := stringField(fields, key); value != "" {
			fields[key] = pseudonymousDurableID(value)
		}
	}
	payload, err := json.Marshal(fields)
	return payload, fields, err
}

func insertSamplingOpportunity(ctx context.Context, tx *sql.Tx, sourceEventID, observerSessionID string, fields map[string]any) error {
	opportunityID := sourceEventID
	targetSessionID := stringField(fields, "target_player_session_id")
	if targetSessionID == "" {
		targetSessionID = stringField(fields, "target_player_id")
	}
	var target any
	if targetSessionID != "" {
		target = targetSessionID
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sampling_opportunities(
			opportunity_id,source_event_id,observer_player_session_id,target_player_session_id,sampling_stream,
			sampling_policy_version,sampling_reason,observer_eligible_count,observer_inclusion_probability,
			target_eligible_count,target_inclusion_probability,risk_set_definition,risk_set_complete,
			queue_admission_probability,scheduler_load_state,queue_delay_ms,drop_reason)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT(opportunity_id) DO NOTHING`,
		opportunityID, sourceEventID, observerSessionID, target, stringField(fields, "sampling_stream"),
		stringField(fields, "sampling_policy_version"), stringField(fields, "sampling_reason"), intField(fields, "observer_eligible_count"), floatField(fields, "observer_inclusion_probability"),
		intField(fields, "target_eligible_count"), floatField(fields, "target_inclusion_probability"), stringField(fields, "risk_set_definition"), boolField(fields, "risk_set_complete"),
		floatField(fields, "queue_admission_probability"), stringField(fields, "scheduler_load_state"), intField(fields, "queue_delay_ms"), stringField(fields, "drop_reason"))
	return err
}

func insertVisibilityProbe(ctx context.Context, tx *sql.Tx, sourceEventID, observerSessionID string, fields map[string]any) error {
	targetSessionID := stringField(fields, "target_player_session_id")
	if targetSessionID == "" {
		targetSessionID = stringField(fields, "target_player_id")
	}
	if targetSessionID == "" {
		return fmt.Errorf("visibility probe %s has no target identity", sourceEventID)
	}
	evidence, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO visibility_probe_runs(
			source_event_id,observer_player_session_id,target_player_session_id,visibility_policy_version,
			observer_origin_mode,derived_class,blocker_categories,probe_queued_ms,probe_started_ms,probe_completed_ms,raw_evidence)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(source_event_id) DO NOTHING`, sourceEventID, observerSessionID, targetSessionID,
		stringField(fields, "visibility_policy_version"), stringField(fields, "observer_origin_mode"), stringField(fields, "classification"),
		stringField(fields, "blocker_type"), intField(fields, "probe_queued_ms"), intField(fields, "probe_started_ms"), intField(fields, "probe_completed_ms"), evidence)
	return err
}

func pseudonymousSessionID(raw string) string {
	return identity.SessionID(raw)
}

func pseudonymousDurableID(raw string) string {
	return identity.DurableID(raw)
}

func rawDurableID(sessionID string) string {
	if index := strings.LastIndex(sessionID, ":"); index >= 0 {
		return sessionID[index+1:]
	}
	return sessionID
}

func digest(value string) string {
	return identity.MustDigest(value)
}

func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func floatField(fields map[string]any, key string) float64 {
	value, _ := fields[key].(float64)
	return value
}

func intField(fields map[string]any, key string) int64 { return int64(floatField(fields, key)) }

func boolField(fields map[string]any, key string) bool {
	switch value := fields[key].(type) {
	case bool:
		return value
	case float64:
		return value != 0
	default:
		return false
	}
}

func sequenceBounds(events []schema.Event) (uint64, uint64) {
	if len(events) == 0 {
		return 0, 0
	}
	first, last := events[0].ServerSequence, events[0].ServerSequence
	for _, event := range events[1:] {
		if event.ServerSequence < first {
			first = event.ServerSequence
		}
		if event.ServerSequence > last {
			last = event.ServerSequence
		}
	}
	return first, last
}
