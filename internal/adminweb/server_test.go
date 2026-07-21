package adminweb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/adminauth"
	"github.com/rogersau/dayz-behaviour/internal/explorer"
	"github.com/rogersau/dayz-behaviour/internal/mapview"
	"github.com/rogersau/dayz-behaviour/internal/review"
)

type verifierStub struct{}

func (verifierStub) Verify(context.Context, url.Values, string) (string, error) {
	return "76561198000000000", nil
}

func TestMapRoutesUseConfiguredSource(t *testing.T) {
	auth, _ := adminauth.New(adminauth.Config{PublicBaseURL: "http://example.test", AdminSteamIDs: []string{"76561198000000000"}, SessionSecret: []byte(strings.Repeat("x", 32))}, verifierStub{})
	requestKey := mapview.TileRequest{MapName: "chernarusplus", Layer: "topographic", Zoom: 1, X: 0, Y: 1}
	maps := &mapview.MemorySource{MapValues: []mapview.Map{{Name: "chernarusplus", Size: 15360, Zoom: 6}}, TileValues: map[mapview.TileRequest]mapview.Tile{requestKey: {Contents: []byte("tile"), ContentType: "image/webp"}}}
	handler, err := New(Config{Explorer: &explorer.MemoryRepository{}, Review: review.NewMemoryRepository(nil, nil), Auth: auth, BearerToken: "machine-secret", PublicURL: "http://example.test", MapSource: maps, DefaultMap: "chernarusplus"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/map/maps", "/v1/map/tiles/chernarusplus/topographic/1/0/1.webp"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer machine-secret")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, response.Code, response.Body.String())
		}
	}
}

func TestExplorerEndpointsRequireAuthenticationAndReturnTimeline(t *testing.T) {
	auth, err := adminauth.New(adminauth.Config{
		PublicBaseURL: "http://example.test", AdminSteamIDs: []string{"76561198000000000"},
		SessionSecret: []byte(strings.Repeat("x", 32)),
	}, verifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	repository := &explorer.MemoryRepository{
		Sessions: []explorer.Session{{PlayerSessionID: "ps_example", ServerID: "server-1", ServerSessionID: "run-1", StartedMS: 1000, EventCount: 1}},
		Entries:  map[string][]explorer.Entry{"ps_example": {{SourceEventID: "event-1", EventType: "WEAPON_FIRED", Authority: "A", ServerTimeMS: 1500, PlayerSessionID: "ps_example", Summary: "Weapon fired", Payload: map[string]any{}}}},
	}
	handler, err := New(Config{Explorer: repository, Review: review.NewMemoryRepository(nil, nil), Auth: auth, BearerToken: "machine-secret", PublicURL: "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}

	unauthorised := httptest.NewRecorder()
	handler.ServeHTTP(unauthorised, httptest.NewRequest(http.MethodGet, "/v1/explore/sessions", nil))
	if unauthorised.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorised status = %d", unauthorised.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/explore/timeline?player_session_id=ps_example", nil)
	request.Header.Set("Authorization", "Bearer machine-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "WEAPON_FIRED") {
		t.Fatalf("timeline status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAdminPageHasStrictAssetPolicy(t *testing.T) {
	auth, _ := adminauth.New(adminauth.Config{PublicBaseURL: "http://example.test", AdminSteamIDs: []string{"76561198000000000"}, SessionSecret: []byte(strings.Repeat("x", 32))}, verifierStub{})
	handler, err := New(Config{Explorer: &explorer.MemoryRepository{}, Review: review.NewMemoryRepository(nil, nil), Auth: auth, PublicURL: "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Header().Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatalf("status = %d, CSP = %q", response.Code, response.Header().Get("Content-Security-Policy"))
	}
}
