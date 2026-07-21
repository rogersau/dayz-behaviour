CREATE TABLE IF NOT EXISTS replay_runs (
    replay_run_id text PRIMARY KEY,
    status text NOT NULL,
    requested_by text NOT NULL,
    diagnostics jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS algorithm_runs (
    algorithm_run_id text PRIMARY KEY,
    status text NOT NULL,
    algorithm_versions jsonb NOT NULL,
    diagnostics jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
