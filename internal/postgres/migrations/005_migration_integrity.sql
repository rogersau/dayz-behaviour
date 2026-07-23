ALTER TABLE schema_migrations
    ADD COLUMN IF NOT EXISTS checksum_sha256 text NOT NULL DEFAULT '';
