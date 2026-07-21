package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/adminauth"
	"github.com/rogersau/dayz-behaviour/internal/adminweb"
	"github.com/rogersau/dayz-behaviour/internal/explorer"
	"github.com/rogersau/dayz-behaviour/internal/mapview"
	"github.com/rogersau/dayz-behaviour/internal/postgres"
	"github.com/rogersau/dayz-behaviour/internal/review"
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
	ctx := context.Background()
	migrationStore, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := migrationStore.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	if err := migrationStore.Close(); err != nil {
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
	address := os.Getenv("DBA_REVIEW_ADDR")
	if address == "" {
		address = "127.0.0.1:8082"
	}
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("admin explorer listening on %s", address)
	log.Fatal(server.ListenAndServe())
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
