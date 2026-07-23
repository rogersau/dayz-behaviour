ALTER TABLE schema_migrations
    ADD COLUMN IF NOT EXISTS checksum_sha256 text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS pseudonym_policy_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    policy_version text NOT NULL,
    key_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
