package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"

	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

var (
	ErrAlreadyStored = errors.New("telemetry batch already stored")
	ErrBatchConflict = errors.New("telemetry batch id conflicts with different content")
	safePathPart     = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

const ManifestFileName = "manifest.ndjson"

type manifestEntry struct {
	Path string `json:"path"`
}

// RawStore persists each batch as a separate immutable JSON file and appends a
// durable relative-path entry to a manifest. The manifest allows continuous
// normalisation to consume new batches without repeatedly walking the entire
// raw tree; exact replay can still scan immutable batch files directly.
type RawStore struct {
	root         string
	manifestPath string
	mu           sync.Mutex
}

func NewRawStore(root string) (*RawStore, error) {
	if root == "" {
		return nil, errors.New("raw storage root is required")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create raw storage root: %w", err)
	}
	return &RawStore{root: root, manifestPath: filepath.Join(root, ManifestFileName)}, nil
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
	relativePath, err := filepath.Rel(s.root, finalPath)
	if err != nil {
		return fmt.Errorf("derive batch manifest path: %w", err)
	}
	if _, err := os.Stat(finalPath); err == nil {
		existing, readErr := os.ReadFile(finalPath)
		if readErr != nil {
			return fmt.Errorf("read existing batch: %w", readErr)
		}
		if bytes.Equal(bytes.TrimSpace(existing), data) {
			if err := s.appendManifest(relativePath); err != nil {
				return fmt.Errorf("repair manifest entry: %w", err)
			}
			return ErrAlreadyStored
		}
		return ErrBatchConflict
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
			existing, readErr := os.ReadFile(finalPath)
			if readErr == nil && bytes.Equal(bytes.TrimSpace(existing), data) {
				if manifestErr := s.appendManifest(relativePath); manifestErr != nil {
					return fmt.Errorf("repair manifest entry: %w", manifestErr)
				}
				return ErrAlreadyStored
			}
			return ErrBatchConflict
		}
		return fmt.Errorf("commit telemetry batch: %w", err)
	}
	removeTemp = false

	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("sync batch directory: %w", err)
	}
	if err := s.appendManifest(relativePath); err != nil {
		return fmt.Errorf("append raw manifest: %w", err)
	}
	return nil
}

func (s *RawStore) appendManifest(relativePath string) error {
	entry, err := json.Marshal(manifestEntry{Path: filepath.ToSlash(relativePath)})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.manifestPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(entry, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func sanitise(value string) string {
	cleaned := safePathPart.ReplaceAllString(value, "_")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "unknown"
	}
	return cleaned
}

func syncDirectory(path string) error {
	// Windows does not allow os.Open on a directory to be flushed with
	// File.Sync. The batch file itself has already been fsynced and atomically
	// renamed at this point; directory fsync is an additional durability step
	// available on Unix filesystems.
	if runtime.GOOS == "windows" {
		return nil
	}

	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
