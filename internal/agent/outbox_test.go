package agent

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestOutboxPersistsAndDeduplicatesBatches(t *testing.T) {
	dataDir := t.TempDir()
	outbox, err := OpenOutbox(dataDir, "server-one", 1024*1024, 100)
	if err != nil {
		t.Fatalf("OpenOutbox: %v", err)
	}
	batch := agentTestBatch()
	if err := outbox.Put(batch); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := outbox.Put(batch); !errors.Is(err, storage.ErrAlreadyStored) {
		t.Fatalf("duplicate Put error = %v", err)
	}

	reopened, err := OpenOutbox(dataDir, "server-one", 1024*1024, 100)
	if err != nil {
		t.Fatalf("reopen outbox: %v", err)
	}
	stats := reopened.Stats()
	if stats.Batches != 1 || stats.Bytes == 0 {
		t.Fatalf("reopened stats = %+v", stats)
	}
	item, ok := reopened.Peek()
	if !ok {
		t.Fatal("expected queued item")
	}
	data, err := reopened.Read(item)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	var stored schema.Batch
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decode stored batch: %v", err)
	}
	if stored.ServerSessionID != batch.ServerSessionID || stored.BatchSequence != batch.BatchSequence {
		t.Fatalf("stored batch = %+v", stored)
	}

	conflicting := batch
	conflicting.ServerTimeMS++
	if err := reopened.Put(conflicting); !errors.Is(err, storage.ErrBatchConflict) {
		t.Fatalf("conflicting Put error = %v", err)
	}
	if err := reopened.Remove(item); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if stats := reopened.Stats(); stats.Batches != 0 || stats.Bytes != 0 {
		t.Fatalf("stats after remove = %+v", stats)
	}
}

func TestOutboxRejectsWrongServerAndCapacityOverflow(t *testing.T) {
	outbox, err := OpenOutbox(t.TempDir(), "server-one", 1, 1)
	if err != nil {
		t.Fatalf("OpenOutbox: %v", err)
	}
	wrongServer := agentTestBatch()
	wrongServer.ServerID = "server-two"
	if err := outbox.Put(wrongServer); !errors.Is(err, ErrServerIDMismatch) {
		t.Fatalf("wrong server error = %v", err)
	}
	if err := outbox.Put(agentTestBatch()); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("capacity error = %v", err)
	}
}

func agentTestBatch() schema.Batch {
	return schema.Batch{
		SchemaVersion:   schema.Version1,
		ServerID:        "server-one",
		ServerSessionID: "session-one",
		BatchSequence:   1,
		ServerTimeMS:    100,
		Events: []schema.Event{{
			EventType:      "PLAYER_SNAPSHOT",
			Source:         schema.SourceServer,
			ServerSequence: 1,
			ServerTimeMS:   100,
		}},
	}
}
