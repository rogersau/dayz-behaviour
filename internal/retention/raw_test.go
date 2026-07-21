package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneRawDefaultsToReportOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "old.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	stats, err := PruneRaw(root, time.Now().Add(-24*time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.FilesExpired != 1 || stats.FilesDeleted != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("dry run removed file")
	}
}
