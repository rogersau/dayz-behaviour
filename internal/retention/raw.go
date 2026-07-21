package retention

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RawStats struct {
	FilesScanned int   `json:"files_scanned"`
	FilesExpired int   `json:"files_expired"`
	FilesDeleted int   `json:"files_deleted"`
	BytesExpired int64 `json:"bytes_expired"`
}

func PruneRaw(root string, before time.Time, execute bool) (RawStats, error) {
	var stats RawStats
	if strings.TrimSpace(root) == "" {
		return stats, fmt.Errorf("raw root is required")
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		stats.FilesScanned++
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.ModTime().Before(before) {
			return nil
		}
		stats.FilesExpired++
		stats.BytesExpired += info.Size()
		if execute {
			if err := os.Remove(path); err != nil {
				return err
			}
			stats.FilesDeleted++
		}
		return nil
	})
	return stats, err
}
