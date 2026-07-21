package adminweb

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rogersau/dayz-behaviour/internal/adminauth"
	"github.com/rogersau/dayz-behaviour/internal/explorer"
	"github.com/rogersau/dayz-behaviour/internal/mapview"
	"github.com/rogersau/dayz-behaviour/internal/review"
)

//go:embed static/*
var staticFiles embed.FS

type Config struct {
	Explorer    explorer.Repository
	Review      review.Repository
	Auth        *adminauth.Manager
	BearerToken string
	PublicURL   string
	MapSource   mapview.Source
	DefaultMap  string
}

func New(config Config) (http.Handler, error) {
	if config.Explorer == nil || config.Review == nil || config.Auth == nil {
		return nil, errors.New("explorer, review repository and auth manager are required")
	}
	publicURL, err := url.Parse(config.PublicURL)
	if err != nil || publicURL.Scheme == "" || publicURL.Host == "" {
		return nil, errors.New("public URL must be absolute")
	}
	server := &server{explorer: config.Explorer, auth: config.Auth, bearerToken: config.BearerToken, publicURL: publicURL, mapSource: config.MapSource, defaultMap: config.DefaultMap}
	reviewAPI, err := review.NewAPI(config.Review, "")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	config.Auth.Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /{$}", server.index)
	mux.HandleFunc("GET /assets/app.css", server.asset("static/app.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /assets/map.css", server.asset("static/map.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /assets/app.js", server.asset("static/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /assets/map.js", server.asset("static/map.js", "text/javascript; charset=utf-8"))
	mux.Handle("GET /v1/explore/sessions", server.authorise(http.HandlerFunc(server.sessions)))
	mux.Handle("GET /v1/explore/timeline", server.authorise(http.HandlerFunc(server.timeline)))
	mux.Handle("GET /v1/map/maps", server.authorise(http.HandlerFunc(server.maps)))
	mux.Handle("GET /v1/map/tiles/{map}/{layer}/{zoom}/{x}/{y}", server.authorise(http.HandlerFunc(server.tile)))
	mux.Handle("/v1/", server.authorise(reviewAPI.Handler()))
	return securityHeaders(mux), nil
}

type server struct {
	explorer    explorer.Repository
	auth        *adminauth.Manager
	bearerToken string
	publicURL   *url.URL
	mapSource   mapview.Source
	defaultMap  string
}

func (s *server) index(w http.ResponseWriter, _ *http.Request) {
	s.serveAsset(w, "static/index.html", "text/html; charset=utf-8")
}

func (s *server) asset(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { s.serveAsset(w, name, contentType) }
}

func (s *server) serveAsset(w http.ResponseWriter, name, contentType string) {
	contents, err := staticFiles.ReadFile(name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(contents)
}

func (s *server) sessions(w http.ResponseWriter, request *http.Request) {
	limit, ok := parseInt(request.URL.Query().Get("limit"), 100)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be numeric"})
		return
	}
	values, err := s.explorer.ListSessions(request.Context(), explorer.SessionQuery{ServerID: request.URL.Query().Get("server"), Search: request.URL.Query().Get("search"), Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "timeline repository unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": values})
}

func (s *server) timeline(w http.ResponseWriter, request *http.Request) {
	query := explorer.TimelineQuery{PlayerSessionID: request.URL.Query().Get("player_session_id")}
	var ok bool
	query.Limit, ok = parseInt(request.URL.Query().Get("limit"), 1000)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be numeric"})
		return
	}
	if query.FromMS, ok = parseOptionalInt64(request.URL.Query().Get("from_ms")); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from_ms must be numeric"})
		return
	}
	if query.ToMS, ok = parseOptionalInt64(request.URL.Query().Get("to_ms")); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to_ms must be numeric"})
		return
	}
	value, err := s.explorer.Timeline(request.Context(), query)
	if errors.Is(err, explorer.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "player session not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "timeline repository unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *server) maps(w http.ResponseWriter, request *http.Request) {
	if s.mapSource == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "map source is not configured"})
		return
	}
	values, err := s.mapSource.Maps(request.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "map source unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"maps": values, "default_map": s.defaultMap})
}

func (s *server) tile(w http.ResponseWriter, request *http.Request) {
	if s.mapSource == nil {
		http.NotFound(w, request)
		return
	}
	zoom, zoomOK := parseInt(request.PathValue("zoom"), -1)
	x, xOK := parseInt(request.PathValue("x"), -1)
	yText := strings.TrimSuffix(request.PathValue("y"), ".webp")
	y, yOK := parseInt(yText, -1)
	if !zoomOK || !xOK || !yOK {
		http.NotFound(w, request)
		return
	}
	value, err := s.mapSource.Tile(request.Context(), mapview.TileRequest{MapName: request.PathValue("map"), Layer: request.PathValue("layer"), Zoom: zoom, X: x, Y: y})
	if errors.Is(err, mapview.ErrNotFound) {
		http.NotFound(w, request)
		return
	}
	if err != nil {
		http.Error(w, "map source unavailable", http.StatusBadGateway)
		return
	}
	if value.ETag != "" {
		w.Header().Set("ETag", value.ETag)
		if request.Header.Get("If-None-Match") == value.ETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Type", value.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(value.Contents)
}

func (s *server) authorise(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if s.validBearer(request) {
			next.ServeHTTP(w, request)
			return
		}
		if _, ok := s.auth.SteamID(request); !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead && !sameOrigin(request, s.publicURL) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid origin"})
			return
		}
		next.ServeHTTP(w, request)
	})
}

func (s *server) validBearer(request *http.Request) bool {
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	value := strings.TrimPrefix(header, prefix)
	return s.bearerToken != "" && strings.HasPrefix(header, prefix) && len(value) == len(s.bearerToken) && subtle.ConstantTimeCompare([]byte(value), []byte(s.bearerToken)) == 1
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self' https://steamcommunity.com")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, request)
	})
}

func sameOrigin(request *http.Request, expected *url.URL) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme == expected.Scheme && parsed.Host == expected.Host
}

func parseInt(value string, fallback int) (int, bool) {
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func parseOptionalInt64(value string) (*int64, bool) {
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return &parsed, err == nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
