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
	"time"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

const defaultMaxRequestBytes int64 = 2 * 1024 * 1024

type BatchStore interface {
	Put(batch schema.Batch) error
}

type Config struct {
	BearerToken     string
	QueryToken      string
	MaxRequestBytes int64
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
}

type Server struct {
	config Config
	store  BatchStore
	logger *slog.Logger
	mux    *http.ServeMux
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
	s.mux.HandleFunc("POST /v1/telemetry/batches", s.handleBatch)
}

func (s *Server) handleBatch(w http.ResponseWriter, request *http.Request) {
	if !s.authorised(request.Header.Get("Authorization"), request.URL.Query().Get("token")) {
		writeError(w, http.StatusUnauthorized, "unauthorised", "valid ingest token required")
		return
	}

	request.Body = http.MaxBytesReader(w, request.Body, s.config.MaxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	var batch schema.Batch
	if err := decoder.Decode(&batch); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "telemetry batch exceeds configured limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := batch.Validate(); err != nil {
		status := http.StatusUnprocessableEntity
		code := "invalid_batch"
		if errors.Is(err, schema.ErrUnsupportedVersion) {
			code = "unsupported_schema_version"
		}
		writeError(w, status, code, err.Error())
		return
	}

	if err := s.store.Put(batch); err != nil {
		if errors.Is(err, storage.ErrAlreadyStored) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":         "duplicate",
				"batch_sequence": batch.BatchSequence,
			})
			return
		}
		s.logger.Error("store telemetry batch", "error", err, "server_id", batch.ServerID, "batch_sequence", batch.BatchSequence)
		writeError(w, http.StatusInternalServerError, "storage_failed", "batch was not durably stored")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":         "stored",
		"batch_sequence": batch.BatchSequence,
	})
}

func (s *Server) authorised(header, queryToken string) bool {
	if s.config.BearerToken == "" && s.config.QueryToken == "" {
		return true
	}
	if s.config.BearerToken != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(header, prefix) {
			provided := strings.TrimPrefix(header, prefix)
			if secureEqual(provided, s.config.BearerToken) {
				return true
			}
		}
	}
	return s.config.QueryToken != "" && secureEqual(queryToken, s.config.QueryToken)
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
