package privacy

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

type BatchRef struct {
	ServerID        string
	ServerSessionID string
	BatchSequence   uint64
}

type ScrubStats struct {
	FilesScanned    int
	FilesRewritten  int
	FilesDeleted    int
	EventsRemoved   int
	AffectedBatches []BatchRef
}

// ScrubRaw removes every event attributable to a durable DayZ identity. An
// event is removed when the subject owns it or appears in any durable/session
// identity role in its payload. A dry run performs the same scan without
// changing files.
func ScrubRaw(root, durablePlayerID string, execute bool) (ScrubStats, error) {
	var stats ScrubStats
	if strings.TrimSpace(root) == "" || strings.TrimSpace(durablePlayerID) == "" {
		return stats, fmt.Errorf("raw root and durable player identity are required")
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		stats.FilesScanned++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var batch schema.Batch
		if err := json.Unmarshal(data, &batch); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		kept := make([]schema.Event, 0, len(batch.Events))
		removed := 0
		for _, event := range batch.Events {
			if eventMatches(event, durablePlayerID) {
				removed++
				continue
			}
			kept = append(kept, event)
		}
		if removed == 0 {
			return nil
		}
		stats.EventsRemoved += removed
		stats.AffectedBatches = append(stats.AffectedBatches, BatchRef{batch.ServerID, batch.ServerSessionID, batch.BatchSequence})
		if !execute {
			return nil
		}
		if len(kept) == 0 {
			if err := os.Remove(path); err != nil {
				return err
			}
			stats.FilesDeleted++
			return nil
		}
		batch.Events = kept
		encoded, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		if err := replaceFile(path, encoded); err != nil {
			return err
		}
		stats.FilesRewritten++
		return nil
	})
	return stats, err
}

func eventMatches(event schema.Event, identity string) bool {
	if event.PlayerID == identity || rawDurableID(event.PlayerSessionID) == identity {
		return true
	}
	var payload any
	if len(event.Payload) == 0 || json.Unmarshal(event.Payload, &payload) != nil {
		return false
	}
	return containsIdentity(payload, identity)
}

func containsIdentity(value any, identity string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if text, ok := child.(string); ok && identityFieldMatches(key, text, identity) {
				return true
			}
			if containsIdentity(child, identity) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsIdentity(child, identity) {
				return true
			}
		}
	}
	return false
}

func identityFieldMatches(key, value, identity string) bool {
	lower := strings.ToLower(key)
	if strings.HasSuffix(lower, "player_session_id") || strings.HasSuffix(lower, "session_id") {
		return rawDurableID(value) == identity
	}
	if strings.HasSuffix(lower, "player_id") || strings.HasSuffix(lower, "durable_player_id") {
		return value == identity
	}
	return false
}

func rawDurableID(sessionID string) string {
	if index := strings.LastIndex(sessionID, ":"); index >= 0 {
		return sessionID[index+1:]
	}
	return sessionID
}

func replaceFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".privacy-rewrite-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	backup := path + ".privacy-backup"
	_ = os.Remove(backup)
	if err := os.Rename(path, backup); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	return os.Remove(backup)
}
