package ingest_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/ingest"
	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestLocalReceiverRejectsUnexpectedServerID(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server, err := ingest.NewServer(ingest.LocalQueryConfig("[REDACTED_SECRET]", "test-server", 2*1024*1024), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	batch := validBatch()
	batch.ServerID = "wrong-server"
	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches?token=[REDACTED_SECRET]", bytes.NewReader(marshalBatch(t, batch)))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
}

func TestIngestReportsTemporaryStorageCapacity(t *testing.T) {
	server, err := ingest.NewServer(ingest.Config{}, capacityStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(marshalBatch(t, validBatch())))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	if response.Header().Get("Retry-After") != "5" {
		t.Fatalf("Retry-After = %q", response.Header().Get("Retry-After"))
	}
}

type capacityStore struct{}

func (capacityStore) Put(schema.Batch) error {
	return storage.ErrCapacityExceeded
}
