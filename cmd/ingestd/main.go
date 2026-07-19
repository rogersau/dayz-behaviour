package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/ingest"
	"github.com/rogersau/dayz-behaviour/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	store, err := storage.NewRawStore(env("DBA_RAW_DIR", "./data/raw"))
	if err != nil {
		logger.Error("initialise raw storage", "error", err)
		os.Exit(1)
	}

	server, err := ingest.NewServer(ingest.Config{
		BearerToken:     os.Getenv("DBA_BEARER_TOKEN"),
		QueryToken:      os.Getenv("DBA_QUERY_TOKEN"),
		MaxRequestBytes: envInt64("DBA_MAX_REQUEST_BYTES", 2*1024*1024),
	}, store, logger)
	if err != nil {
		logger.Error("initialise ingest server", "error", err)
		os.Exit(1)
	}

	httpServer := server.HTTPServer(env("DBA_LISTEN_ADDR", "127.0.0.1:8080"))

	errChannel := make(chan error, 1)
	go func() {
		logger.Info("ingest server listening", "address", httpServer.Addr)
		errChannel <- httpServer.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("ingest server failed", "error", err)
			os.Exit(1)
		}
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt64(name string, fallback int64) int64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
