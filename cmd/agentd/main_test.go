package main

import (
	"encoding/base64"
	"testing"
)

func TestRandomCredentialUsesThirtyTwoRandomBytes(t *testing.T) {
	first, err := randomCredential()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomCredential()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("generated credentials unexpectedly matched")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("decode generated credential: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded credential length = %d, want 32", len(decoded))
	}
}
