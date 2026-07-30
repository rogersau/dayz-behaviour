package ingest_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/ingest"
	"github.com/rogersau/dayz-behaviour/internal/storage"
)

func TestIngestBindsServerCredentialToServerID(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server, err := ingest.NewServer(ingest.Config{
		ServerCredentials: map[string]string{"test-server": "[REDACTED_SECRET]"},
	}, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(marshalBatch(t, validBatch())))
	request.Header.Set("Authorization", "Bearer [REDACTED_SECRET]")
	request.Header.Set("X-DayZ-Server-ID", "test-server")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("accepted status = %d; body=%s", response.Code, response.Body.String())
	}

	wrongBody := validBatch()
	wrongBody.ServerID = "another-server"
	request = httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(marshalBatch(t, wrongBody)))
	request.Header.Set("Authorization", "Bearer [REDACTED_SECRET]")
	request.Header.Set("X-DayZ-Server-ID", "test-server")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("mismatch status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
}

func TestIngestRejectsUnknownServerCredential(t *testing.T) {
	store, err := storage.NewRawStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server, err := ingest.NewServer(ingest.Config{
		ServerCredentials: map[string]string{"test-server": "[REDACTED_SECRET]"},
	}, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/telemetry/batches", bytes.NewReader(marshalBatch(t, validBatch())))
	request.Header.Set("Authorization", "Bearer wrong-value")
	request.Header.Set("X-DayZ-Server-ID", "test-server")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
