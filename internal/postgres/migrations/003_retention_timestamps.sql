ALTER TABLE normalized_events ADD COLUMN IF NOT EXISTS normalized_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE feature_results ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS normalized_events_normalized_at_idx ON normalized_events(normalized_at);
CREATE INDEX IF NOT EXISTS feature_results_created_at_idx ON feature_results(created_at);
