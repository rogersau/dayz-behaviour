package replay

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

type Sink interface {
	Accept(context.Context, schema.Batch) error
}

type SinkFunc func(context.Context, schema.Batch) error

func (f SinkFunc) Accept(ctx context.Context, batch schema.Batch) error {
	return f(ctx, batch)
}

type Stats struct {
	Batches int
	Events  int
}

type checkpoint struct {
	ManifestOffset int64 `json:"manifest_offset"`
}

type manifestEntry struct {
	Path string `json:"path"`
}

func Run(ctx context.Context, rawRoot string, sink Sink) (Stats, error) {
	return runWalk(ctx, rawRoot, sink, nil)
}

// Tracker performs one complete replay when first constructed, then consumes
// only new manifest entries for the rest of the process lifetime. A checkpoint
// tracker persists the manifest byte offset so restarts avoid a full tree walk.
type Tracker struct {
	seen           map[string]struct{}
	checkpointPath string
	manifestOffset int64
	initialized    bool
}

func NewTracker() *Tracker {
	return &Tracker{seen: make(map[string]struct{})}
}

func NewCheckpointTracker(path string) (*Tracker, error) {
	tracker := &Tracker{seen: make(map[string]struct{}), checkpointPath: path}
	if strings.TrimSpace(path) == "" {
		return tracker, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tracker, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read replay checkpoint: %w", err)
	}
	var value checkpoint
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode replay checkpoint: %w", err)
	}
	if value.ManifestOffset < 0 {
		return nil, errors.New("replay checkpoint manifest offset must be non-negative")
	}
	tracker.manifestOffset = value.ManifestOffset
	tracker.initialized = true
	return tracker, nil
}

func (t *Tracker) Run(ctx context.Context, rawRoot string, sink Sink) (Stats, error) {
	if t == nil {
		return Stats{}, errors.New("replay tracker is required")
	}
	if strings.TrimSpace(rawRoot) == "" {
		return Stats{}, errors.New("raw root is required")
	}
	if sink == nil {
		return Stats{}, errors.New("replay sink is required")
	}
	manifestPath := filepath.Join(rawRoot, storage.ManifestFileName)
	if !t.initialized {
		initialManifestSize := int64(0)
		if info, err := os.Stat(manifestPath); err == nil {
			initialManifestSize = info.Size()
		} else if !errors.Is(err, os.ErrNotExist) {
			return Stats{}, fmt.Errorf("stat raw manifest: %w", err)
		}
		stats, err := runWalk(ctx, rawRoot, sink, t.seen)
		if err != nil {
			return stats, err
		}
		t.manifestOffset = initialManifestSize
		t.initialized = true
		if err := t.saveCheckpoint(); err != nil {
			return stats, err
		}
		return stats, nil
	}
	if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
		return runWalk(ctx, rawRoot, sink, t.seen)
	} else if err != nil {
		return Stats{}, fmt.Errorf("stat raw manifest: %w", err)
	}
	return t.runManifest(ctx, rawRoot, manifestPath, sink)
}

func (t *Tracker) runManifest(ctx context.Context, rawRoot, manifestPath string, sink Sink) (Stats, error) {
	var stats Stats
	file, err := os.Open(manifestPath)
	if err != nil {
		return stats, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return stats, err
	}
	if t.manifestOffset > info.Size() {
		t.manifestOffset = 0
	}
	if _, err := file.Seek(t.manifestOffset, io.SeekStart); err != nil {
		return stats, err
	}
	reader := bufio.NewReader(file)
	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return stats, readErr
		}
		if errors.Is(readErr, io.EOF) && !strings.HasSuffix(line, "\n") {
			return stats, nil
		}
		if strings.TrimSpace(line) != "" {
			var entry manifestEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				return stats, fmt.Errorf("decode raw manifest at offset %d: %w", t.manifestOffset, err)
			}
			path, err := safeManifestPath(rawRoot, entry.Path)
			if err != nil {
				return stats, fmt.Errorf("raw manifest at offset %d: %w", t.manifestOffset, err)
			}
			if _, ok := t.seen[path]; !ok {
				batchStats, err := processBatchFile(ctx, path, sink)
				if err != nil {
					return stats, err
				}
				stats.Batches += batchStats.Batches
				stats.Events += batchStats.Events
				t.seen[path] = struct{}{}
			}
		}
		t.manifestOffset += int64(len(line))
		if err := t.saveCheckpoint(); err != nil {
			return stats, err
		}
		if errors.Is(readErr, io.EOF) {
			return stats, nil
		}
	}
}

func safeManifestPath(root, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("manifest path is empty")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("manifest path escapes raw root")
	}
	if !strings.EqualFold(filepath.Ext(clean), ".json") {
		return "", errors.New("manifest path is not a JSON batch")
	}
	return filepath.Join(root, clean), nil
}

func (t *Tracker) saveCheckpoint() error {
	if strings.TrimSpace(t.checkpointPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(t.checkpointPath), 0o750); err != nil {
		return fmt.Errorf("create replay checkpoint directory: %w", err)
	}
	data, err := json.Marshal(checkpoint{ManifestOffset: t.manifestOffset})
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(t.checkpointPath), ".replay-checkpoint-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o640); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, t.checkpointPath); err != nil {
		return err
	}
	return nil
}

func runWalk(ctx context.Context, rawRoot string, sink Sink, seen map[string]struct{}) (Stats, error) {
	if strings.TrimSpace(rawRoot) == "" {
		return Stats{}, errors.New("raw root is required")
	}
	if sink == nil {
		return Stats{}, errors.New("replay sink is required")
	}
	var stats Stats
	err := filepath.WalkDir(rawRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		if _, ok := seen[path]; ok {
			return nil
		}
		batchStats, err := processBatchFile(ctx, path, sink)
		if err != nil {
			return err
		}
		if seen != nil {
			seen[path] = struct{}{}
		}
		stats.Batches += batchStats.Batches
		stats.Events += batchStats.Events
		return nil
	})
	if err != nil {
		return stats, err
	}
	return stats, nil
}

func processBatchFile(ctx context.Context, path string, sink Sink) (Stats, error) {
	var stats Stats
	if err := ctx.Err(); err != nil {
		return stats, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return stats, fmt.Errorf("read raw batch %q: %w", path, err)
	}
	var batch schema.Batch
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&batch); err != nil {
		return stats, fmt.Errorf("decode raw batch %q: %w", path, err)
	}
	if err := batch.Validate(); err != nil {
		return stats, fmt.Errorf("validate raw batch %q: %w", path, err)
	}
	if err := sink.Accept(ctx, batch); err != nil {
		return stats, fmt.Errorf("replay raw batch %q: %w", path, err)
	}
	stats.Batches = 1
	stats.Events = len(batch.Events)
	return stats, nil
}
