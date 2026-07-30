package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerAuthMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server-auth.json")
	if err := os.WriteFile(path, []byte(`{"server-one":"[REDACTED_SECRET]"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	authMap, err := loadServerAuthMap(path)
	if err != nil {
		t.Fatalf("loadServerAuthMap: %v", err)
	}
	if authMap["server-one"] != "[REDACTED_SECRET]" {
		t.Fatalf("auth map = %#v", authMap)
	}
}

func TestLoadServerAuthMapRejectsEmptyMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server-auth.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadServerAuthMap(path); err == nil {
		t.Fatal("expected empty map to be rejected")
	}
}
