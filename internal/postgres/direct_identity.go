package postgres

import (
	"context"
	"fmt"

	"github.com/rogersau/dayz-behaviour/internal/identity"
)

var directIdentityTables = []string{
	"review_dispositions",
	"review_cases",
	"candidate_rankings",
	"feature_results",
	"matched_control_sets",
	"analysis_observations",
	"decision_windows",
	"cue_facts",
	"observer_target_episodes",
	"visibility_probe_runs",
	"sampling_opportunities",
	"encounters",
	"algorithm_runs",
	"replay_runs",
	"normalized_events",
	"player_sessions",
}

// DirectIdentityRebuildScope returns exact row counts for the normalized and
// review tables that will be rebuilt. Restricted raw batches and privacy audit
// records are not removed.
func (s *Store) DirectIdentityRebuildScope(ctx context.Context) (map[string]int64, error) {
	counts := make(map[string]int64, len(directIdentityTables))
	for _, table := range directIdentityTables {
		var count int64
		if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}

// ResetForDirectIdentities clears derived data and switches the database policy
// to direct identities. The caller must replay restricted raw batches before
// running analysis again.
func (s *Store) ResetForDirectIdentities(ctx context.Context) (map[string]int64, error) {
	counts, err := s.DirectIdentityRebuildScope(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, table := range directIdentityTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return nil, fmt.Errorf("clear %s: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pseudonym_policy_state(singleton,policy_version,key_id)
		VALUES(true,$1,$2)
		ON CONFLICT(singleton) DO UPDATE SET policy_version=EXCLUDED.policy_version,key_id=EXCLUDED.key_id`,
		identity.DirectPolicyVersion, identity.DirectKeyID); err != nil {
		return nil, fmt.Errorf("set direct identity policy: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return counts, nil
}
