package identity

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	DirectPolicyVersion = "direct-identifiers-v1"
	DirectKeyID         = "not-applicable"
)

type Policy struct {
	Version string
	KeyID   string
}

func CurrentPolicy() (Policy, error) {
	return Policy{Version: DirectPolicyVersion, KeyID: DirectKeyID}, nil
}

// Digest remains available for internal opaque record IDs. It is not used for
// player or player-session identity values.
func Digest(value string) (string, error) {
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
	return raw
}

func SessionID(raw string) string {
	return raw
}
