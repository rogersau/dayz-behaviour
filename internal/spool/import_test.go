package spool_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/spool"
	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestImportDirStoresAndArchivesSpool(t *testing.T) {
	spoolDir := t.TempDir()
	rawDir := t.TempDir()
	store, err := storage.NewRawStore(rawDir)
	if err != nil {
		t.Fatal(err)
	}
	batch := validBatch()
	data, _ := json.Marshal(batch)
	path := filepath.Join(spoolDir, "failed-batches-0.ndjson")
	if err := os.WriteFile(path, append(append(data, '\n'), append(data, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	stats, err := spool.ImportDir(context.Background(), spoolDir, store, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 1 || stats.Batches != 2 || stats.Duplicates != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("source spool still exists: %v", err)
	}
	archiveEntries, err := os.ReadDir(filepath.Join(spoolDir, "archive"))
	if err != nil || len(archiveEntries) != 1 {
		t.Fatalf("archive entries=%v err=%v", archiveEntries, err)
	}
}

func TestImportDirRestoresMalformedSpool(t *testing.T) {
	spoolDir := t.TempDir()
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(spoolDir, "failed-batches-0.ndjson")
	if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	_ = os.Chtimes(path, old, old)

	if _, err := spool.ImportDir(context.Background(), spoolDir, store, time.Second); err == nil {
		t.Fatal("expected malformed spool to fail")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source spool was not restored: %v", err)
	}
}

func validBatch() schema.Batch {
	return schema.Batch{
		SchemaVersion:   1,
		ServerID:        "server",
		ServerSessionID: "session",
		BatchSequence:   1,
		ServerTimeMS:    1,
		Events: []schema.Event{{
			EventType:       "PLAYER_SNAPSHOT",
			Source:          schema.SourceServer,
			SourceAuthority: schema.AuthorityServer,
			ServerSequence:  1,
			ServerTimeMS:    1,
		}},
	}
}
