package replay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

func Run(ctx context.Context, rawRoot string, sink Sink) (Stats, error) {
	return run(ctx, rawRoot, sink, nil)
}

// Tracker replays each immutable raw batch once for the lifetime of a
// normalizer process. A restarted process safely scans everything again; the
// PostgreSQL sink is idempotent.
type Tracker struct {
	seen map[string]struct{}
}

func NewTracker() *Tracker {
	return &Tracker{seen: make(map[string]struct{})}
}

func (t *Tracker) Run(ctx context.Context, rawRoot string, sink Sink) (Stats, error) {
	if t == nil {
		return Stats{}, errors.New("replay tracker is required")
	}
	return run(ctx, rawRoot, sink, t.seen)
}

func run(ctx context.Context, rawRoot string, sink Sink, seen map[string]struct{}) (Stats, error) {
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

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read raw batch %q: %w", path, err)
		}
		var batch schema.Batch
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&batch); err != nil {
			return fmt.Errorf("decode raw batch %q: %w", path, err)
		}
		if err := batch.Validate(); err != nil {
			return fmt.Errorf("validate raw batch %q: %w", path, err)
		}
		if err := sink.Accept(ctx, batch); err != nil {
			return fmt.Errorf("replay raw batch %q: %w", path, err)
		}
		if seen != nil {
			seen[path] = struct{}{}
		}
		stats.Batches++
		stats.Events += len(batch.Events)
		return nil
	})
	if err != nil {
		return stats, err
	}
	return stats, nil
}
