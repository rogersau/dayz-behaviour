package ingest

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

const defaultMaxRequestBytes int64 = 2 * 1024 * 1024

type BatchStore interface {
	Put(batch schema.Batch) error
}

type Config struct {
	BearerToken       string
	QueryToken        string
	MaxRequestBytes   int64
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ServerCredentials map[string]string
	ExpectedServerID  string
}

func LocalQueryConfig(queryValue, expectedServerID string, maxRequestBytes int64) Config {
	return Config{
		QueryToken:       queryValue,
		MaxRequestBytes:  maxRequestBytes,
		ExpectedServerID: expectedServerID,
	}
}

type Server struct {
	config               Config
	store                BatchStore
	logger               *slog.Logger
	mux                  *http.ServeMux
	requests             atomic.Uint64
	accepted             atomic.Uint64
	duplicates           atomic.Uint64
	rejected             atomic.Uint64
	storageFailures      atomic.Uint64
	authFailures         atomic.Uint64
	decodeFailures       atomic.Uint64
	validationFailures   atomic.Uint64
	bytesReceived        atomic.Uint64
	requestMicroseconds  atomic.Uint64
	storageMicroseconds  atomic.Uint64
	spoolFilesImported   atomic.Uint64
	spoolBatchesImported atomic.Uint64
}

func NewServer(config Config, store BatchStore, logger *slog.Logger) (*Server, error) {
	if store == nil {
		return nil, errors.New("batch store is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if config.MaxRequestBytes <= 0 {
		config.MaxRequestBytes = defaultMaxRequestBytes
	}
	server := &Server{
		config: config,
		store:  store,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) HTTPServer(address string) *http.Server {
	readTimeout := defaultDuration(s.config.ReadTimeout, 10*time.Second)
	writeTimeout := defaultDuration(s.config.WriteTimeout, 10*time.Second)
	idleTimeout := defaultDuration(s.config.IdleTimeout, 60*time.Second)
	return &http.Server{
		Addr:              address,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	s.mux.HandleFunc("GET /metrics", s.handleMetrics)
	s.mux.HandleFunc("POST /v1/telemetry/batches", s.handleBatch)
}

func (s *Server) handleBatch(w http.ResponseWriter, request *http.Request) {
	started := time.Now()
	defer func() { s.requestMicroseconds.Add(uint64(time.Since(started).Microseconds())) }()
	s.requests.Add(1)
	boundServerID, authorised := s.authorised(
		request.Header.Get("Authorization"),
		request.URL.Query().Get("token"),
		request.Header.Get("X-DayZ-Server-ID"),
	)
	if !authorised {
		s.rejected.Add(1)
		s.authFailures.Add(1)
		writeError(w, http.StatusUnauthorized, "unauthorised", "valid ingest credential required")
		return
	}

	request.Body = http.MaxBytesReader(w, request.Body, s.config.MaxRequestBytes)
	defer request.Body.Close()

	counter := &countingReader{reader: request.Body}
	defer func() { s.bytesReceived.Add(counter.count) }()
	decoder := json.NewDecoder(counter)
	decoder.DisallowUnknownFields()

	var batch schema.Batch
	if err := decoder.Decode(&batch); err != nil {
		s.decodeFailures.Add(1)
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			s.rejected.Add(1)
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "telemetry batch exceeds configured limit")
			return
		}
		s.rejected.Add(1)
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := ensureEOF(decoder); err != nil {
		s.decodeFailures.Add(1)
		s.rejected.Add(1)
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := batch.Validate(); err != nil {
		s.validationFailures.Add(1)
		s.rejected.Add(1)
		status := http.StatusUnprocessableEntity
		code := "invalid_batch"
		if errors.Is(err, schema.ErrUnsupportedVersion) {
			code = "unsupported_schema_version"
		}
		writeError(w, status, code, err.Error())
		return
	}
	if s.config.ExpectedServerID != "" && batch.ServerID != s.config.ExpectedServerID {
		s.rejected.Add(1)
		writeError(w, http.StatusForbidden, "server_id_mismatch", "batch server_id does not match this receiver")
		return
	}
	if boundServerID != "" && batch.ServerID != boundServerID {
		s.rejected.Add(1)
		s.authFailures.Add(1)
		writeError(w, http.StatusForbidden, "server_id_mismatch", "credential is not authorised for this server_id")
		return
	}

	storageStarted := time.Now()
	err := s.store.Put(batch)
	s.storageMicroseconds.Add(uint64(time.Since(storageStarted).Microseconds()))
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyStored) {
			s.duplicates.Add(1)
			writeJSON(w, http.StatusOK, map[string]any{
				"status":         "duplicate",
				"batch_sequence": batch.BatchSequence,
			})
			return
		}
		if errors.Is(err, storage.ErrBatchConflict) {
			s.rejected.Add(1)
			writeError(w, http.StatusConflict, "batch_conflict", "batch identity already exists with different content")
			return
		}
		if errors.Is(err, storage.ErrCapacityExceeded) {
			s.storageFailures.Add(1)
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusServiceUnavailable, "storage_capacity_exceeded", "durable telemetry storage is at capacity")
			return
		}
		s.storageFailures.Add(1)
		s.logger.Error("store telemetry batch", "error", err, "server_id", batch.ServerID, "batch_sequence", batch.BatchSequence)
		writeError(w, http.StatusInternalServerError, "storage_failed", "batch was not durably stored")
		return
	}

	s.accepted.Add(1)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":         "stored",
		"batch_sequence": batch.BatchSequence,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "dba_ingest_requests_total %d\n", s.requests.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_accepted_total %d\n", s.accepted.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_duplicates_total %d\n", s.duplicates.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_rejected_total %d\n", s.rejected.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_storage_failures_total %d\n", s.storageFailures.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_auth_failures_total %d\n", s.authFailures.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_decode_failures_total %d\n", s.decodeFailures.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_validation_failures_total %d\n", s.validationFailures.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_bytes_received_total %d\n", s.bytesReceived.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_request_duration_microseconds_total %d\n", s.requestMicroseconds.Load())
	_, _ = fmt.Fprintf(w, "dba_ingest_storage_duration_microseconds_total %d\n", s.storageMicroseconds.Load())
	_, _ = fmt.Fprintf(w, "dba_spool_files_imported_total %d\n", s.spoolFilesImported.Load())
	_, _ = fmt.Fprintf(w, "dba_spool_batches_imported_total %d\n", s.spoolBatchesImported.Load())
}

func (s *Server) RecordSpoolImport(files, batches int) {
	if files > 0 {
		s.spoolFilesImported.Add(uint64(files))
	}
	if batches > 0 {
		s.spoolBatchesImported.Add(uint64(batches))
	}
}

type countingReader struct {
	reader io.Reader
	count  uint64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	r.count += uint64(count)
	return count, err
}

func (s *Server) authorised(header, queryCredential, claimedServerID string) (string, bool) {
	providedBearer := bearerCredential(header)
	if claimedServerID != "" && len(s.config.ServerCredentials) > 0 {
		expected, ok := s.config.ServerCredentials[claimedServerID]
		if !ok || providedBearer == "" || !secureEqual(providedBearer, expected) {
			return "", false
		}
		return claimedServerID, true
	}
	if s.config.BearerToken == "" && s.config.QueryToken == "" {
		return "", len(s.config.ServerCredentials) == 0
	}
	if s.config.BearerToken != "" && providedBearer != "" && secureEqual(providedBearer, s.config.BearerToken) {
		return "", true
	}
	return "", s.config.QueryToken != "" && secureEqual(queryCredential, s.config.QueryToken)
}

func bearerCredential(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimPrefix(header, prefix)
}

func secureEqual(provided, expected string) bool {
	if len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("request body contains multiple JSON values")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		_, _ = fmt.Fprintln(w, `{"error":{"code":"response_write_failed","message":"failed to encode response"}}`)
	}
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
