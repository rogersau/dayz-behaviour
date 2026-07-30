package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploaderRemovesAcceptedBatch(t *testing.T) {
	var received bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		received = true
		if request.Header.Get("Authorization") != "Bearer [REDACTED_SECRET]" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("X-DayZ-Server-ID") != "server-one" {
			t.Errorf("X-DayZ-Server-ID = %q", request.Header.Get("X-DayZ-Server-ID"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	outbox, err := OpenOutbox(t.TempDir(), "server-one", 1024*1024, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := outbox.Put(agentTestBatch()); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.ServerID = "server-one"
	config.UpstreamURL = upstream.URL
	config.UpstreamBearerToken = "[REDACTED_SECRET]"
	uploader := NewUploader(config, outbox, nil)

	processed, err := uploader.uploadNext(context.Background())
	if err != nil {
		t.Fatalf("uploadNext: %v", err)
	}
	if !processed || !received {
		t.Fatalf("processed=%v received=%v", processed, received)
	}
	if stats := outbox.Stats(); stats.Batches != 0 {
		t.Fatalf("queue stats = %+v", stats)
	}
	if status := uploader.Status(); status.UploadedBatches != 1 || status.LastSuccessAt == nil {
		t.Fatalf("upload status = %+v", status)
	}
}

func TestUploaderRetainsRetryableFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	outbox, err := OpenOutbox(t.TempDir(), "server-one", 1024*1024, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := outbox.Put(agentTestBatch()); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.ServerID = "server-one"
	config.UpstreamURL = upstream.URL
	config.UpstreamBearerToken = "[REDACTED_SECRET]"
	uploader := NewUploader(config, outbox, nil)

	processed, err := uploader.uploadNext(context.Background())
	if err == nil || processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if stats := outbox.Stats(); stats.Batches != 1 || stats.DeadLetters != 0 {
		t.Fatalf("queue stats = %+v", stats)
	}
}

func TestUploaderDeadLettersPermanentRejection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid batch", http.StatusUnprocessableEntity)
	}))
	defer upstream.Close()

	outbox, err := OpenOutbox(t.TempDir(), "server-one", 1024*1024, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := outbox.Put(agentTestBatch()); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.ServerID = "server-one"
	config.UpstreamURL = upstream.URL
	config.UpstreamBearerToken = "[REDACTED_SECRET]"
	uploader := NewUploader(config, outbox, nil)

	processed, err := uploader.uploadNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if stats := outbox.Stats(); stats.Batches != 0 || stats.DeadLetters != 1 {
		t.Fatalf("queue stats = %+v", stats)
	}
}
