ALTER TABLE schema_migrations
    ADD COLUMN IF NOT EXISTS checksum_sha256 text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS pseudonym_policy_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    policy_version text NOT NULL,
    key_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Existing normalized identities were created with the original unkeyed SHA-256
-- policy. Mark that namespace explicitly so configuring a new key fails closed
-- until an operator performs an intentional identity migration or clears the
-- development database. Empty databases remain free to adopt the keyed policy.
INSERT INTO pseudonym_policy_state(singleton, policy_version, key_id)
SELECT true, 'sha256-unkeyed-development-v1', 'legacy-unkeyed'
WHERE EXISTS (SELECT 1 FROM player_sessions LIMIT 1)
ON CONFLICT(singleton) DO NOTHING;
