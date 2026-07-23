package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

const (
	KeyedPolicyVersion  = "hmac-sha256-v1"
	LegacyPolicyVersion = "sha256-unkeyed-development-v1"
	LegacyKeyID         = "legacy-unkeyed"
)

type Policy struct {
	Version string
	KeyID   string
	Secret  []byte
}

func CurrentPolicy() (Policy, error) {
	secret := os.Getenv("DBA_PSEUDONYM_SECRET")
	keyID := strings.TrimSpace(os.Getenv("DBA_PSEUDONYM_KEY_ID"))
	if secret == "" {
		return Policy{Version: LegacyPolicyVersion, KeyID: LegacyKeyID}, nil
	}
	if len(secret) < 32 {
		return Policy{}, errors.New("DBA_PSEUDONYM_SECRET must contain at least 32 bytes")
	}
	if keyID == "" {
		return Policy{}, errors.New("DBA_PSEUDONYM_KEY_ID is required when DBA_PSEUDONYM_SECRET is set")
	}
	return Policy{Version: KeyedPolicyVersion, KeyID: keyID, Secret: []byte(secret)}, nil
}

func Digest(value string) (string, error) {
	policy, err := CurrentPolicy()
	if err != nil {
		return "", err
	}
	if policy.Version == KeyedPolicyVersion {
		mac := hmac.New(sha256.New, policy.Secret)
		_, _ = mac.Write([]byte(value))
		return hex.EncodeToString(mac.Sum(nil)), nil
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:]), nil
}

func MustDigest(value string) string {
	result, err := Digest(value)
	if err != nil {
		panic(err)
	}
	return result
}

func DurableID(raw string) string {
	if raw == "" {
		return ""
	}
	return "dp_" + MustDigest(raw)
}

func SessionID(raw string) string {
	if raw == "" {
		return ""
	}
	return "ps_" + MustDigest(raw)
}
