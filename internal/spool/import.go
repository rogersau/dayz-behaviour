package spool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

type BatchStore interface {
	Put(schema.Batch) error
}

type Stats struct {
	Files      int
	Batches    int
	Duplicates int
}

func ImportDir(ctx context.Context, dir string, store BatchStore, minimumAge time.Duration) (Stats, error) {
	var stats Stats
	if strings.TrimSpace(dir) == "" {
		return stats, errors.New("spool directory is required")
	}
	if store == nil {
		return stats, errors.New("batch store is required")
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return stats, nil
	}
	if err != nil {
		return stats, fmt.Errorf("read spool directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	now := time.Now()
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".ndjson") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return stats, fmt.Errorf("stat spool file %s: %w", entry.Name(), err)
		}
		if minimumAge > 0 && now.Sub(info.ModTime()) < minimumAge {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		claimed := path + ".importing"
		if err := os.Rename(path, claimed); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return stats, fmt.Errorf("claim spool file %s: %w", entry.Name(), err)
		}
		fileStats, importErr := importFile(ctx, claimed, store)
		if importErr != nil {
			if restoreErr := os.Rename(claimed, path); restoreErr != nil {
				return stats, fmt.Errorf("import spool file %s: %v; restore failed: %w", entry.Name(), importErr, restoreErr)
			}
			return stats, fmt.Errorf("import spool file %s: %w", entry.Name(), importErr)
		}
		archiveDir := filepath.Join(dir, "archive")
		if err := os.MkdirAll(archiveDir, 0o750); err != nil {
			_ = os.Rename(claimed, path)
			return stats, fmt.Errorf("create spool archive: %w", err)
		}
		archivePath := filepath.Join(archiveDir, entry.Name()+"."+time.Now().UTC().Format("20060102T150405.000000000Z")+".imported")
		if err := os.Rename(claimed, archivePath); err != nil {
			_ = os.Rename(claimed, path)
			return stats, fmt.Errorf("archive spool file %s: %w", entry.Name(), err)
		}
		stats.Files++
		stats.Batches += fileStats.Batches
		stats.Duplicates += fileStats.Duplicates
	}
	return stats, nil
}

func importFile(ctx context.Context, path string, store BatchStore) (Stats, error) {
	var stats Stats
	file, err := os.Open(path)
	if err != nil {
		return stats, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 256*1024)
	lineNumber := 0
	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		line, readErr := reader.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			lineNumber++
			var batch schema.Batch
			decoder := json.NewDecoder(strings.NewReader(string(line)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&batch); err != nil {
				return stats, fmt.Errorf("line %d decode: %w", lineNumber, err)
			}
			if err := ensureEOF(decoder); err != nil {
				return stats, fmt.Errorf("line %d decode: %w", lineNumber, err)
			}
			if err := batch.Validate(); err != nil {
				return stats, fmt.Errorf("line %d validate: %w", lineNumber, err)
			}
			err := store.Put(batch)
			switch {
			case err == nil:
				stats.Batches++
			case errors.Is(err, storage.ErrAlreadyStored):
				stats.Batches++
				stats.Duplicates++
			default:
				return stats, fmt.Errorf("line %d store: %w", lineNumber, err)
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		return stats, readErr
	}
	if lineNumber == 0 {
		return stats, errors.New("spool file contained no batches")
	}
	return stats, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("line contains multiple JSON values")
}
