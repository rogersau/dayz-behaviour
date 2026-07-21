package storage_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestRawStorePutAndDuplicate(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewRawStore: %v", err)
	}

	batch := validBatch()
	if err := store.Put(batch); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	if err := store.Put(batch); !errors.Is(err, storage.ErrAlreadyStored) {
		t.Fatalf("duplicate Put error = %v, want ErrAlreadyStored", err)
	}
}

func TestRawStoreRejectsConflictingDuplicate(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	batch := validBatch()
	if err := store.Put(batch); err != nil {
		t.Fatal(err)
	}
	batch.ServerTimeMS++
	if err := store.Put(batch); !errors.Is(err, storage.ErrBatchConflict) {
		t.Fatalf("Put conflict error = %v, want ErrBatchConflict", err)
	}
}

func TestRawStoreSanitisesPathIdentifiers(t *testing.T) {
	root := t.TempDir()
	store, err := storage.NewRawStore(root)
	if err != nil {
		t.Fatalf("NewRawStore: %v", err)
	}

	batch := validBatch()
	batch.ServerID = "../../server"
	batch.ServerSessionID = "session/one"
	if err := store.Put(batch); err != nil {
		t.Fatalf("Put: %v", err)
	}

	path := filepath.Join(root, ".._.._server", "session_one", "00000000000000000001.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	var stored schema.Batch
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("Unmarshal stored batch: %v", err)
	}
	if stored.ServerID != batch.ServerID {
		t.Fatalf("stored ServerID = %q, want %q", stored.ServerID, batch.ServerID)
	}
}

func validBatch() schema.Batch {
	payload := json.RawMessage(`{"position":[1,2,3]}`)
	return schema.Batch{
		SchemaVersion:   schema.Version1,
		ServerID:        "test-server",
		ServerSessionID: "test-session",
		BatchSequence:   1,
		ServerTimeMS:    1_700_000_000_000,
		Events: []schema.Event{{
			EventType:      "PLAYER_SNAPSHOT",
			Source:         schema.SourceServer,
			ServerSequence: 1,
			ServerTimeMS:   100,
			Payload:        payload,
		}},
	}
}
