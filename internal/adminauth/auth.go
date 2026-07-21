package adminauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie = "dba_admin_session"
	pendingCookie = "dba_steam_login"
)

type Verifier interface {
	Verify(context.Context, url.Values, string) (string, error)
}

type Config struct {
	PublicBaseURL string
	AdminSteamIDs []string
	SessionSecret []byte
	CookieSecure  bool
	SessionTTL    time.Duration
}

type Manager struct {
	baseURL  *url.URL
	admins   map[string]struct{}
	secret   []byte
	secure   bool
	ttl      time.Duration
	verifier Verifier
	now      func() time.Time
}

func New(config Config, verifier Verifier) (*Manager, error) {
	baseURL, err := url.Parse(strings.TrimRight(config.PublicBaseURL, "/"))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, errors.New("public base URL must be an absolute HTTP(S) URL")
	}
	if baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("public base URL must not contain a query or fragment")
	}
	if len(config.SessionSecret) < 32 {
		return nil, errors.New("session secret must contain at least 32 bytes")
	}
	admins := make(map[string]struct{}, len(config.AdminSteamIDs))
	for _, id := range config.AdminSteamIDs {
		id = strings.TrimSpace(id)
		if !validSteamID(id) {
			return nil, fmt.Errorf("invalid admin Steam ID %q", id)
		}
		admins[id] = struct{}{}
	}
	if len(admins) == 0 {
		return nil, errors.New("at least one admin Steam ID is required")
	}
	if verifier == nil {
		verifier = NewSteamVerifier(nil)
	}
	ttl := config.SessionTTL
	if ttl == 0 {
		ttl = 12 * time.Hour
	}
	if ttl < 0 {
		return nil, errors.New("session TTL must be positive")
	}
	return &Manager{baseURL: baseURL, admins: admins, secret: append([]byte(nil), config.SessionSecret...), secure: config.CookieSecure || baseURL.Scheme == "https", ttl: ttl, verifier: verifier, now: time.Now}, nil
}

func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/steam/login", m.begin)
	mux.HandleFunc("GET /auth/steam/callback", m.complete)
	mux.HandleFunc("POST /auth/logout", m.logout)
	mux.HandleFunc("GET /auth/me", m.me)
}

func (m *Manager) SteamID(request *http.Request) (string, bool) {
	cookie, err := request.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	var session signedValue
	if !m.decode(cookie.Value, &session) || session.ExpiresAt < m.now().Unix() || !validSteamID(session.Value) {
		return "", false
	}
	_, allowed := m.admins[session.Value]
	return session.Value, allowed
}

func (m *Manager) begin(w http.ResponseWriter, request *http.Request) {
	state, err := randomToken()
	if err != nil {
		http.Error(w, "could not start login", http.StatusInternalServerError)
		return
	}
	expires := m.now().Add(10 * time.Minute)
	m.setCookie(w, pendingCookie, m.encode(signedValue{Value: state, ExpiresAt: expires.Unix()}), expires)
	returnTo := m.callbackURL(state)
	query := url.Values{
		"openid.ns":         {"http://specs.openid.net/auth/2.0"},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {returnTo},
		"openid.realm":      {m.baseURL.String() + "/"},
		"openid.identity":   {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.claimed_id": {"http://specs.openid.net/auth/2.0/identifier_select"},
	}
	http.Redirect(w, request, "https://steamcommunity.com/openid/login?"+query.Encode(), http.StatusFound)
}

func (m *Manager) complete(w http.ResponseWriter, request *http.Request) {
	pending, err := request.Cookie(pendingCookie)
	if err != nil {
		m.loginError(w, "login request expired")
		return
	}
	var value signedValue
	if !m.decode(pending.Value, &value) || value.ExpiresAt < m.now().Unix() || !hmac.Equal([]byte(value.Value), []byte(request.URL.Query().Get("state"))) {
		m.loginError(w, "login state is invalid")
		return
	}
	steamID, err := m.verifier.Verify(request.Context(), request.URL.Query(), m.callbackURL(value.Value))
	if err != nil {
		m.loginError(w, "Steam could not verify this login")
		return
	}
	if _, allowed := m.admins[steamID]; !allowed {
		m.loginError(w, "this Steam account is not on the admin allowlist")
		return
	}
	expires := m.now().Add(m.ttl)
	m.setCookie(w, sessionCookie, m.encode(signedValue{Value: steamID, ExpiresAt: expires.Unix()}), expires)
	m.clearCookie(w, pendingCookie)
	http.Redirect(w, request, m.baseURL.String()+"/", http.StatusFound)
}

func (m *Manager) logout(w http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request, m.baseURL) {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	m.clearCookie(w, sessionCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (m *Manager) me(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	steamID, ok := m.SteamID(request)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "steam_id": steamID})
}

func (m *Manager) callbackURL(state string) string {
	return m.baseURL.String() + "/auth/steam/callback?state=" + url.QueryEscape(state)
}

func (m *Manager) loginError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = fmt.Fprintf(w, "<!doctype html><title>Login failed</title><h1>Login failed</h1><p>%s</p><p><a href=\"/\">Return to the admin tool</a></p>", message)
}

func (m *Manager) setCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Expires: expires, MaxAge: int(expires.Sub(m.now()).Seconds()), HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode})
}

func (m *Manager) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode})
}

type signedValue struct {
	Value     string `json:"v"`
	ExpiresAt int64  `json:"exp"`
}

func (m *Manager) encode(value signedValue) string {
	payload, _ := json.Marshal(value)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *Manager) decode(raw string, target *signedValue) bool {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	return err == nil && json.Unmarshal(payload, target) == nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validSteamID(value string) bool {
	if len(value) < 16 || len(value) > 20 {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func sameOrigin(request *http.Request, expected *url.URL) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme == expected.Scheme && parsed.Host == expected.Host
}
