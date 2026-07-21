CREATE TABLE IF NOT EXISTS privacy_audit_log (
    privacy_audit_id text PRIMARY KEY,
    action text NOT NULL,
    subject_pseudonym text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL,
    affected_counts jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS raw_batches_ingested_at_idx ON raw_batches(ingested_at);
CREATE INDEX IF NOT EXISTS candidate_rankings_created_at_idx ON candidate_rankings(created_at);
