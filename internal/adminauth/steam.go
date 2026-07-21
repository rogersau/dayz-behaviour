package adminauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const steamOpenIDEndpoint = "https://steamcommunity.com/openid/login"

type SteamVerifier struct{ client *http.Client }

func NewSteamVerifier(client *http.Client) *SteamVerifier {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &SteamVerifier{client: client}
}

func (v *SteamVerifier) Verify(ctx context.Context, values url.Values, expectedReturnTo string) (string, error) {
	if values.Get("openid.ns") != "http://specs.openid.net/auth/2.0" || values.Get("openid.mode") != "id_res" {
		return "", errors.New("invalid OpenID response mode")
	}
	if values.Get("openid.op_endpoint") != steamOpenIDEndpoint || values.Get("openid.return_to") != expectedReturnTo {
		return "", errors.New("unexpected OpenID endpoint or return URL")
	}
	claimed := values.Get("openid.claimed_id")
	if claimed == "" || claimed != values.Get("openid.identity") {
		return "", errors.New("claimed identity mismatch")
	}
	steamID, err := steamIDFromClaim(claimed)
	if err != nil {
		return "", err
	}
	verification := url.Values{"openid.mode": {"check_authentication"}}
	for key, items := range values {
		if !strings.HasPrefix(key, "openid.") || key == "openid.mode" {
			continue
		}
		for _, item := range items {
			verification.Add(key, item)
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, steamOpenIDEndpoint, strings.NewReader(verification.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := v.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("verify with Steam: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	if err != nil || response.StatusCode != http.StatusOK {
		return "", errors.New("Steam verification failed")
	}
	valid := false
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == "is_valid:true" {
			valid = true
		}
	}
	if !valid {
		return "", errors.New("Steam rejected the OpenID assertion")
	}
	return steamID, nil
}

func steamIDFromClaim(claimed string) (string, error) {
	parsed, err := url.Parse(claimed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host != "steamcommunity.com" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid Steam claimed ID")
	}
	const prefix = "/openid/id/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", errors.New("invalid Steam claimed ID path")
	}
	steamID := strings.TrimPrefix(parsed.Path, prefix)
	if !validSteamID(steamID) {
		return "", errors.New("invalid Steam ID")
	}
	return steamID, nil
}

var _ Verifier = (*SteamVerifier)(nil)
