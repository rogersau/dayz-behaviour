package review

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type SQLRepository struct{ db *sql.DB }

func OpenSQLRepository(ctx context.Context, databaseURL string) (*SQLRepository, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLRepository{db: db}, nil
}

func (r *SQLRepository) Close() error { return r.db.Close() }

func (r *SQLRepository) ListCandidates() ([]Candidate, error) {
	rows, err := r.db.Query(`SELECT candidate_ranking_id,durable_player_id,ranking_policy_version,review_tier,component_values,created_at FROM candidate_rankings ORDER BY created_at DESC,candidate_ranking_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Candidate
	for rows.Next() {
		value, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *SQLRepository) GetCandidate(id string) (Candidate, bool, error) {
	row := r.db.QueryRow(`SELECT candidate_ranking_id,durable_player_id,ranking_policy_version,review_tier,component_values,created_at FROM candidate_rankings WHERE candidate_ranking_id=$1`, id)
	value, err := scanCandidate(row)
	if err == sql.ErrNoRows {
		return Candidate{}, false, nil
	}
	return value, err == nil, err
}

type scanner interface{ Scan(...any) error }

func scanCandidate(row scanner) (Candidate, error) {
	var value Candidate
	var components []byte
	var policy string
	if err := row.Scan(&value.CandidateID, &value.PlayerPseudonym, &policy, &value.ReviewPriority, &components, &value.CreatedAt); err != nil {
		return value, err
	}
	if len(components) > 0 {
		_ = json.Unmarshal(components, &value)
	}
	value.PolicyVersions = ensureMap(value.PolicyVersions)
	value.PolicyVersions["ranking"] = policy
	return value, nil
}

func ensureMap(value map[string]string) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return value
}

func (r *SQLRepository) GetCase(id string) (Case, bool, error) {
	var value Case
	err := r.db.QueryRow(`SELECT review_case_id,candidate_ranking_id,status,created_at FROM review_cases WHERE review_case_id=$1`, id).Scan(&value.CaseID, &value.CandidateID, &value.Status, &value.CreatedAt)
	if err == sql.ErrNoRows {
		return Case{}, false, nil
	}
	return value, err == nil, err
}

func (r *SQLRepository) GetIncident(id string) (Incident, bool, error) {
	var value Incident
	var sourceIDs, evidence []byte
	err := r.db.QueryRow(`SELECT observation_id,source_event_ids,values FROM analysis_observations WHERE observation_id=$1`, id).Scan(&value.IncidentID, &sourceIDs, &evidence)
	if err == sql.ErrNoRows {
		return Incident{}, false, nil
	}
	if err != nil {
		return Incident{}, false, err
	}
	_ = json.Unmarshal(sourceIDs, &value.SourceEventIDs)
	_ = json.Unmarshal(evidence, &value.VisibilityEvidence)
	rows, err := r.db.Query(`SELECT matching_variables,control_quality FROM matched_control_sets WHERE observation_id=$1 ORDER BY matched_control_set_id`, id)
	if err != nil {
		return Incident{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var fields []byte
		var quality float64
		if err := rows.Scan(&fields, &quality); err != nil {
			return Incident{}, false, err
		}
		item := map[string]any{"control_quality": quality}
		_ = json.Unmarshal(fields, &item)
		value.MatchedControls = append(value.MatchedControls, item)
	}
	return value, true, rows.Err()
}

func (r *SQLRepository) AddDisposition(value Disposition) error {
	if value.CaseID == "" || value.ReviewerID == "" || value.Disposition == "" {
		return fmt.Errorf("case_id, reviewer_id and disposition are required")
	}
	id := stableReviewID(value.CaseID, value.ReviewerID, value.CreatedAt.String())
	_, err := r.db.Exec(`INSERT INTO review_dispositions(review_disposition_id,review_case_id,reviewer_id,disposition,notes,created_at) VALUES($1,$2,$3,$4,$5,$6)`, id, value.CaseID, value.ReviewerID, value.Disposition, value.Notes, value.CreatedAt)
	return err
}

func (r *SQLRepository) CreateReplayRun(value ReplayRun) error {
	_, err := r.db.Exec(`INSERT INTO replay_runs(replay_run_id,status,requested_by,created_at) VALUES($1,$2,$3,$4)`, value.RunID, value.Status, value.RequestedBy, value.CreatedAt)
	return err
}

func (r *SQLRepository) GetAlgorithmRun(id string) (AlgorithmRun, bool, error) {
	var value AlgorithmRun
	var versions, diagnostics []byte
	err := r.db.QueryRow(`SELECT algorithm_run_id,status,algorithm_versions,diagnostics,created_at FROM algorithm_runs WHERE algorithm_run_id=$1`, id).Scan(&value.RunID, &value.Status, &versions, &diagnostics, &value.CreatedAt)
	if err == sql.ErrNoRows {
		return AlgorithmRun{}, false, nil
	}
	if err != nil {
		return AlgorithmRun{}, false, err
	}
	_ = json.Unmarshal(versions, &value.AlgorithmVersions)
	_ = json.Unmarshal(diagnostics, &value.Diagnostics)
	return value, true, nil
}

func stableReviewID(parts ...string) string {
	sum := sha256.Sum256([]byte(fmt.Sprint(parts)))
	return "review_" + hex.EncodeToString(sum[:16])
}

var _ Repository = (*SQLRepository)(nil)
