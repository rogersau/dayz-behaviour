package adminauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const testSteamID = "76561198000000000"

type verifierFunc func(context.Context, url.Values, string) (string, error)

func (f verifierFunc) Verify(ctx context.Context, values url.Values, returnTo string) (string, error) {
	return f(ctx, values, returnTo)
}

func TestSteamLoginCreatesAdminSession(t *testing.T) {
	manager, err := New(Config{PublicBaseURL: "http://127.0.0.1:8082", AdminSteamIDs: []string{testSteamID}, SessionSecret: []byte(strings.Repeat("s", 32))}, verifierFunc(func(_ context.Context, _ url.Values, _ string) (string, error) {
		return testSteamID, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	mux := http.NewServeMux()
	manager.Register(mux)

	begin := httptest.NewRecorder()
	mux.ServeHTTP(begin, httptest.NewRequest(http.MethodGet, "/auth/steam/login", nil))
	if begin.Code != http.StatusFound {
		t.Fatalf("begin status = %d, body = %s", begin.Code, begin.Body.String())
	}
	location, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	returnTo, err := url.Parse(location.Query().Get("openid.return_to"))
	if err != nil {
		t.Fatal(err)
	}
	callback := httptest.NewRequest(http.MethodGet, returnTo.RequestURI()+"&openid.mode=id_res", nil)
	for _, cookie := range begin.Result().Cookies() {
		callback.AddCookie(cookie)
	}
	complete := httptest.NewRecorder()
	mux.ServeHTTP(complete, callback)
	if complete.Code != http.StatusFound {
		t.Fatalf("complete status = %d, body = %s", complete.Code, complete.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/explore/sessions", nil)
	for _, cookie := range complete.Result().Cookies() {
		if cookie.Name == sessionCookie {
			request.AddCookie(cookie)
		}
	}
	if id, ok := manager.SteamID(request); !ok || id != testSteamID {
		t.Fatalf("SteamID = %q, %v", id, ok)
	}
}

func TestSteamLoginRejectsNonAdmin(t *testing.T) {
	manager, err := New(Config{PublicBaseURL: "http://example.test", AdminSteamIDs: []string{testSteamID}, SessionSecret: []byte(strings.Repeat("s", 32))}, verifierFunc(func(_ context.Context, _ url.Values, _ string) (string, error) {
		return "76561198000000001", nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	manager.Register(mux)
	begin := httptest.NewRecorder()
	mux.ServeHTTP(begin, httptest.NewRequest(http.MethodGet, "/auth/steam/login", nil))
	location, _ := url.Parse(begin.Header().Get("Location"))
	returnTo, _ := url.Parse(location.Query().Get("openid.return_to"))
	request := httptest.NewRequest(http.MethodGet, returnTo.RequestURI()+"&openid.mode=id_res", nil)
	for _, cookie := range begin.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "not on the admin allowlist") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestSteamClaimValidation(t *testing.T) {
	for _, test := range []struct {
		claim string
		ok    bool
	}{
		{"https://steamcommunity.com/openid/id/" + testSteamID, true},
		{"http://steamcommunity.com/openid/id/" + testSteamID, true},
		{"https://evil.example/openid/id/" + testSteamID, false},
		{"https://steamcommunity.com/openid/id/not-a-number", false},
	} {
		_, err := steamIDFromClaim(test.claim)
		if (err == nil) != test.ok {
			t.Fatalf("steamIDFromClaim(%q) error = %v", test.claim, err)
		}
	}
}
