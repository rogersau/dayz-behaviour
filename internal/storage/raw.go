package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

var (
	ErrAlreadyStored = errors.New("telemetry batch already stored")
	safePathPart     = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

// RawStore persists each batch as a separate immutable JSON file. This is
// deliberately simple for the feasibility spike and provides exact replay.
type RawStore struct {
	root string
	mu   sync.Mutex
}

func NewRawStore(root string) (*RawStore, error) {
	if root == "" {
		return nil, errors.New("raw storage root is required")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create raw storage root: %w", err)
	}
	return &RawStore{root: root}, nil
}

func (s *RawStore) Put(batch schema.Batch) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal telemetry batch: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(
		s.root,
		sanitise(batch.ServerID),
		sanitise(batch.ServerSessionID),
	)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create batch directory: %w", err)
	}

	finalPath := filepath.Join(dir, fmt.Sprintf("%020d.json", batch.BatchSequence))
	if _, err := os.Stat(finalPath); err == nil {
		return ErrAlreadyStored
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat batch path: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".batch-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary batch file: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		_ = temp.Close()
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := temp.Chmod(0o640); err != nil {
		return fmt.Errorf("set temporary batch permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary batch: %w", err)
	}
	if _, err := temp.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write batch newline: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary batch: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary batch: %w", err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		if _, statErr := os.Stat(finalPath); statErr == nil {
			return ErrAlreadyStored
		}
		return fmt.Errorf("commit telemetry batch: %w", err)
	}
	removeTemp = false

	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("sync batch directory: %w", err)
	}
	return nil
}

func sanitise(value string) string {
	cleaned := safePathPart.ReplaceAllString(value, "_")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "unknown"
	}
	return cleaned
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
