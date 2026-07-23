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
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	batch := schema.Batch{SchemaVersion: 1, ServerID: "server", ServerSessionID: "session", BatchSequence: 1, ServerTimeMS: 1, Events: []schema.Event{{EventType: "PLAYER_SNAPSHOT", Source: schema.SourceServer, SourceAuthority: schema.AuthorityServer, ServerSequence: 1, ServerTimeMS: 1}}}
	data, _ := json.Marshal(batch)
	path := filepath.Join(spoolDir, "failed-batches-0.ndjson")
	if err := os.WriteFile(path, append(append(data, '\n'), append(data, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	_ = os.Chtimes(path, old, old)
	stats, err := spool.ImportDir(context.Background(), spoolDir, store, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 1 || stats.Batches != 2 || stats.Duplicates != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if entries, err := os.ReadDir(filepath.Join(spoolDir, "archive")); err != nil || len(entries) != 1 {
		t.Fatalf("archive entries=%v err=%v", entries, err)
	}
}
