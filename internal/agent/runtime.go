package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/ingest"
	"github.com/rogersau/dayz-behaviour/internal/spool"
)

type Runtime struct {
	config    Config
	outbox    *Outbox
	uploader  *Uploader
	logger    *slog.Logger
	startedAt time.Time
}

func NewRuntime(config Config, logger *slog.Logger) (*Runtime, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	outbox, err := OpenOutbox(config.DataDir, config.ServerID, config.MaxQueueBytes, config.MaxQueueBatches)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		config:    config,
		outbox:    outbox,
		uploader:  NewUploader(config, outbox, logger),
		logger:    logger,
		startedAt: time.Now().UTC(),
	}, nil
}

func (r *Runtime) Run(ctx context.Context) error {
	ingestConfig := ingest.LocalQueryConfig(r.config.LocalIngestToken, r.config.ServerID, 2*1024*1024)
	ingestServer, err := ingest.NewServer(ingestConfig, r.outbox, r.logger)
	if err != nil {
		return fmt.Errorf("initialise local ingest server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", r.handleReady)
	mux.HandleFunc("GET /agent/status", r.handleStatus)
	mux.HandleFunc("GET /agent/metrics", r.handleMetrics)
	mux.Handle("/", ingestServer.Handler())

	httpServer := &http.Server{
		Addr:              r.config.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	go r.uploader.Run(runContext)
	if r.config.DayZSpoolDir != "" {
		go r.runSpoolImporter(runContext, ingestServer)
	}

	errChannel := make(chan error, 1)
	go func() {
		r.logger.Info("DayZ Behaviour agent listening", "address", httpServer.Addr, "server_id", r.config.ServerID, "upstream", r.config.UpstreamURL)
		errChannel <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case serveErr := <-errChannel:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			cancel()
			return fmt.Errorf("local ingest server failed: %w", serveErr)
		}
	}

	cancel()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shut down local ingest server: %w", err)
	}
	return nil
}

func (r *Runtime) handleReady(w http.ResponseWriter, _ *http.Request) {
	if r.outbox.AtCapacity() {
		writeAgentJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "outbox_full"})
		return
	}
	writeAgentJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (r *Runtime) handleStatus(w http.ResponseWriter, _ *http.Request) {
	queue := r.outbox.Stats()
	upload := r.uploader.Status()
	status := "ok"
	if r.outbox.AtCapacity() || (queue.Batches > 0 && upload.LastError != "") || queue.DeadLetters > 0 {
		status = "degraded"
	}
	writeAgentJSON(w, http.StatusOK, map[string]any{
		"status":        status,
		"version":       Version,
		"server_id":     r.config.ServerID,
		"started_at":    r.startedAt,
		"queue":         queue,
		"upstream":      upload,
		"spool_enabled": r.config.DayZSpoolDir != "",
	})
}

func (r *Runtime) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	queue := r.outbox.Stats()
	upload := r.uploader.Status()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "dba_agent_queue_batches %d\n", queue.Batches)
	_, _ = fmt.Fprintf(w, "dba_agent_queue_bytes %d\n", queue.Bytes)
	_, _ = fmt.Fprintf(w, "dba_agent_dead_letters_total %d\n", queue.DeadLetters)
	_, _ = fmt.Fprintf(w, "dba_agent_uploaded_batches_total %d\n", upload.UploadedBatches)
	_, _ = fmt.Fprintf(w, "dba_agent_upload_failures_total %d\n", upload.UploadFailures)
}

func (r *Runtime) runSpoolImporter(ctx context.Context, ingestServer *ingest.Server) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		stats, err := spool.ImportDir(ctx, r.config.DayZSpoolDir, r.outbox, 5*time.Second)
		if err != nil && !errors.Is(err, context.Canceled) {
			r.logger.Error("import DayZ spool", "error", err)
		}
		if stats.Files > 0 {
			ingestServer.RecordSpoolImport(stats.Files, stats.Batches)
			r.logger.Info("imported DayZ spool", "files", stats.Files, "batches", stats.Batches, "duplicates", stats.Duplicates)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func writeAgentJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
