package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/identity"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestIntegrationMigrateAndAcceptAreIdempotent(t *testing.T) {
	databaseURL := os.Getenv("DBA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DBA_TEST_DATABASE_URL is not configured")
	}
	if os.Getenv("DBA_TEST_DATABASE_RESET") != "true" {
		t.Skip("DBA_TEST_DATABASE_RESET=true is required for destructive integration setup")
	}
	t.Setenv("DBA_PSEUDONYM_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DBA_PSEUDONYM_KEY_ID", "integration-v1")

	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	batch := schema.Batch{
		SchemaVersion: 1,
		ServerID: "server",
		ServerSessionID: "session",
		BatchSequence: 1,
		ServerTimeMS: 1_000,
		Events: []schema.Event{{
			EventType: "PLAYER_SNAPSHOT",
			Source: schema.SourceServer,
			SourceAuthority: schema.AuthorityServer,
			SourceEventID: "session:1",
			ServerSequence: 1,
			ServerTimeMS: 1_000,
			PlayerSessionID: "session:1:76561198000000000",
		}},
	}
	if err := store.Accept(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if err := store.Accept(ctx, batch); err != nil {
		t.Fatal(err)
	}

	var rawCount, eventCount, blankChecksums int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM raw_batches`).Scan(&rawCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM normalized_events`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE checksum_sha256=''`).Scan(&blankChecksums); err != nil {
		t.Fatal(err)
	}
	if rawCount != 1 || eventCount != 1 || blankChecksums != 0 {
		t.Fatalf("raw=%d events=%d blank_checksums=%d", rawCount, eventCount, blankChecksums)
	}

	var durableID, policyVersion, keyID string
	if err := store.db.QueryRowContext(ctx, `SELECT durable_player_id FROM player_sessions LIMIT 1`).Scan(&durableID); err != nil {
		t.Fatal(err)
	}
	if durableID != identity.DurableID("76561198000000000") {
		t.Fatalf("durable ID = %q", durableID)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT policy_version,key_id FROM pseudonym_policy_state WHERE singleton=true`).Scan(&policyVersion, &keyID); err != nil {
		t.Fatal(err)
	}
	if policyVersion != identity.KeyedPolicyVersion || keyID != "integration-v1" {
		t.Fatalf("policy=%s key=%s", policyVersion, keyID)
	}

	t.Setenv("DBA_PSEUDONYM_KEY_ID", "different-key")
	if err := store.Migrate(ctx); err == nil {
		t.Fatal("expected pseudonym key mismatch to fail closed")
	}
}
