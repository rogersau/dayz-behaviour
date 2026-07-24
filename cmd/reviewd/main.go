package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/adminauth"
	"github.com/rogersau/dayz-behaviour/internal/adminweb"
	"github.com/rogersau/dayz-behaviour/internal/explorer"
	"github.com/rogersau/dayz-behaviour/internal/mapview"
	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/replay"
	"github.com/rogersau/dayz-behaviour/internal/review"
)

const (
	defaultNormalizationInterval = 5 * time.Second
	shutdownTimeout              = 10 * time.Second
)

func main() {
	databaseURL := os.Getenv("DBA_DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DBA_DATABASE_URL is required")
	}
	if os.Getenv("DBA_REVIEW_TOKEN") == "" {
		log.Fatal("DBA_REVIEW_TOKEN is required")
	}
	publicURL := os.Getenv("DBA_PUBLIC_BASE_URL")
	if publicURL == "" {
		publicURL = "http://127.0.0.1:8082"
	}
	auth, err := adminauth.New(adminauth.Config{
		PublicBaseURL: publicURL,
		AdminSteamIDs: splitCommaSeparated(os.Getenv("DBA_STEAM_ADMIN_IDS")),
		SessionSecret: []byte(os.Getenv("DBA_SESSION_SECRET")),
	}, nil)
	if err != nil {
		log.Fatalf("configure Steam admin authentication: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	normalizationStore, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := normalizationStore.Close(); err != nil {
			log.Printf("close normalization store: %v", err)
		}
	}()
	if err := normalizationStore.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	repository, err := review.OpenSQLRepository(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	explorerRepository, err := explorer.OpenSQLRepository(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer explorerRepository.Close()
	var mapSource mapview.Source
	if mapDirectory := os.Getenv("DBA_MAP_DIR"); mapDirectory != "" {
		mapSource, err = mapview.OpenFilesystem(mapDirectory)
		if err != nil {
			log.Fatalf("configure local maps: %v", err)
		}
	}
	handler, err := adminweb.New(adminweb.Config{
		Explorer: explorerRepository, Review: repository, Auth: auth,
		BearerToken: os.Getenv("DBA_REVIEW_TOKEN"), PublicURL: publicURL,
		MapSource: mapSource, DefaultMap: os.Getenv("DBA_DEFAULT_MAP"),
	})
	if err != nil {
		log.Fatal(err)
	}

	var workers sync.WaitGroup
	if rawRoot := strings.TrimSpace(os.Getenv("DBA_RAW_DIR")); rawRoot != "" {
		interval, err := parseNormalizationInterval(os.Getenv("DBA_NORMALIZE_INTERVAL"))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("continuous normalization enabled: raw_dir=%s interval=%s", rawRoot, interval)
		workers.Add(1)
		go func() {
			defer workers.Done()
			runNormalizer(ctx, rawRoot, interval, normalizationStore)
		}()
	}

	address := os.Getenv("DBA_REVIEW_ADDR")
	if address == "" {
		address = "127.0.0.1:8082"
	}
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("admin explorer listening on %s", address)
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.ListenAndServe()
	}()

	var serveErr error
	select {
	case serveErr = <-serveErrors:
		stop()
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("admin explorer shutdown failed: %v", err)
			if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				log.Printf("force close admin explorer: %v", closeErr)
			}
		}
		cancel()
		serveErr = <-serveErrors
	}

	workers.Wait()
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		log.Fatal(serveErr)
	}
}

func parseNormalizationInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultNormalizationInterval, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse DBA_NORMALIZE_INTERVAL %q: %w", value, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("DBA_NORMALIZE_INTERVAL must be positive, got %q", value)
	}
	return interval, nil
}

func runNormalizer(ctx context.Context, rawRoot string, interval time.Duration, sink replay.Sink) {
	tracker := replay.NewTracker()
	run := func() {
		stats, err := tracker.Run(ctx, rawRoot, sink)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("normalization failed: %v", err)
			}
			return
		}
		if stats.Batches > 0 {
			log.Printf("normalization complete: batches=%d events=%d", stats.Batches, stats.Events)
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func splitCommaSeparated(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
