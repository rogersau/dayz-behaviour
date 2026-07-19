package ingest_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/ingest"
	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

func TestIngestAuthorisationAndIdempotency(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewRawStore: %v", err)
	}
	server, err := ingest.NewServer(ingest.Config{BearerToken: "secret"}, store, slog.New(slog.NewTextHandler(testWriter{t}, nil)))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body := marshalBatch(t, validBatch())

	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorised status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	for index, expected := range []int{http.StatusAccepted, http.StatusOK} {
		request = httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer secret")
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != expected {
			t.Fatalf("request %d status = %d, want %d; body=%s", index, response.Code, expected, response.Body.String())
		}
	}
}

func TestIngestAcceptsDayZQueryToken(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewRawStore: %v", err)
	}
	server, err := ingest.NewServer(ingest.Config{QueryToken: "dayz-secret"}, store, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches?token=dayz-secret", bytes.NewReader(marshalBatch(t, validBatch())))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusAccepted, response.Body.String())
	}
}

func TestIngestRejectsUnknownFields(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewRawStore: %v", err)
	}
	server, err := ingest.NewServer(ingest.Config{}, store, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body := []byte(`{"schema_version":1,"server_id":"x","server_session_id":"y","batch_sequence":1,"server_time_ms":1,"events":[],"unexpected":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestIngestRejectsInvalidBatch(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewRawStore: %v", err)
	}
	server, err := ingest.NewServer(ingest.Config{}, store, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	batch := validBatch()
	batch.Events = nil
	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(marshalBatch(t, batch)))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
}

func validBatch() schema.Batch {
	return schema.Batch{
		SchemaVersion:   schema.Version1,
		ServerID:        "test-server",
		ServerSessionID: "session-one",
		BatchSequence:   1,
		ServerTimeMS:    1_700_000_000_000,
		Events: []schema.Event{{
			EventType:      "PLAYER_SNAPSHOT",
			Source:         schema.SourceServer,
			ServerSequence: 1,
			ServerTimeMS:   100,
		}},
	}
}

func marshalBatch(t *testing.T, batch schema.Batch) []byte {
	t.Helper()
	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return data
}

type testWriter struct{ t *testing.T }

func (writer testWriter) Write(data []byte) (int, error) {
	writer.t.Log(string(data))
	return len(data), nil
}
