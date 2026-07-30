package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigResolvesRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")
	config := DefaultConfig()
	config.ServerID = "server-one"
	config.LocalIngestToken = "[REDACTED_SECRET]"
	config.UpstreamURL = "http://127.0.0.1:9090"
	config.UpstreamBearerToken = "[REDACTED_SECRET]"
	config.DataDir = "./state"
	config.DayZSpoolDir = "./profiles/DayZBehaviourProbe/spool"
	if err := SaveConfig(path, config); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.DataDir != filepath.Join(dir, "state") {
		t.Fatalf("DataDir = %q", loaded.DataDir)
	}
	if loaded.DayZSpoolDir != filepath.Join(dir, "profiles", "DayZBehaviourProbe", "spool") {
		t.Fatalf("DayZSpoolDir = %q", loaded.DayZSpoolDir)
	}
	if loaded.UpstreamEndpoint() != "http://127.0.0.1:9090/v1/telemetry/batches" {
		t.Fatalf("UpstreamEndpoint = %q", loaded.UpstreamEndpoint())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
	}
}

func TestConfigRejectsUnsafeNetworkBoundaries(t *testing.T) {
	config := DefaultConfig()
	config.ServerID = "server-one"
	config.LocalIngestToken = "[REDACTED_SECRET]"
	config.UpstreamBearerToken = "[REDACTED_SECRET]"

	config.ListenAddr = "0.0.0.0:8080"
	config.UpstreamURL = "https://example.test"
	if err := config.Validate(); err == nil {
		t.Fatal("expected non-loopback listen address to be rejected")
	}

	config.ListenAddr = "127.0.0.1:8080"
	config.UpstreamURL = "http://example.test"
	if err := config.Validate(); err == nil {
		t.Fatal("expected non-TLS remote upstream to be rejected")
	}
}
