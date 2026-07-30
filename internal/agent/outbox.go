package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rogersau/dayz-behaviour/internal/storage"
	"github.com/rogersau/dayz-behaviour/pkg/schema"
)

var (
	ErrQueueFull        = storage.ErrCapacityExceeded
	ErrServerIDMismatch = errors.New("batch server_id does not match agent configuration")
	agentSafePathPart   = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

type QueueItem struct {
	path string
	size int64
}

type QueueStats struct {
	Batches     int   `json:"batches"`
	Bytes       int64 `json:"bytes"`
	DeadLetters int   `json:"dead_letters"`
	MaxBatches  int   `json:"max_batches"`
	MaxBytes    int64 `json:"max_bytes"`
}

type queuedFile struct {
	path    string
	size    int64
	modTime time.Time
}

type Outbox struct {
	root          string
	deadLetterDir string
	serverID      string
	maxBytes      int64
	maxBatches    int

	mu          sync.Mutex
	items       []queuedFile
	known       map[string]struct{}
	bytes       int64
	deadLetters int
	wake        chan struct{}
}

func OpenOutbox(dataDir, serverID string, maxBytes int64, maxBatches int) (*Outbox, error) {
	if dataDir == "" || serverID == "" {
		return nil, errors.New("outbox data directory and server ID are required")
	}
	if maxBytes <= 0 || maxBatches <= 0 {
		return nil, errors.New("outbox limits must be greater than zero")
	}
	outbox := &Outbox{
		root:          filepath.Join(dataDir, "outbox"),
		deadLetterDir: filepath.Join(dataDir, "dead-letter"),
		serverID:      serverID,
		maxBytes:      maxBytes,
		maxBatches:    maxBatches,
		known:         make(map[string]struct{}),
		wake:          make(chan struct{}, 1),
	}
	if err := os.MkdirAll(outbox.root, 0o750); err != nil {
		return nil, fmt.Errorf("create outbox: %w", err)
	}
	if err := os.MkdirAll(outbox.deadLetterDir, 0o750); err != nil {
		return nil, fmt.Errorf("create dead-letter directory: %w", err)
	}
	if err := outbox.scan(); err != nil {
		return nil, err
	}
	return outbox, nil
}

func (o *Outbox) Put(batch schema.Batch) error {
	if batch.ServerID != o.serverID {
		return fmt.Errorf("%w: got %q, want %q", ErrServerIDMismatch, batch.ServerID, o.serverID)
	}
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal queued batch: %w", err)
	}
	data = append(data, '\n')
	path := o.batchPath(batch)

	o.mu.Lock()
	defer o.mu.Unlock()
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, data) {
			return storage.ErrAlreadyStored
		}
		return storage.ErrBatchConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect queued batch: %w", err)
	}
	if len(o.items) >= o.maxBatches || o.bytes+int64(len(data)) > o.maxBytes {
		return ErrQueueFull
	}
	if err := atomicWrite(path, data); err != nil {
		return err
	}
	item := queuedFile{path: path, size: int64(len(data)), modTime: time.Now()}
	o.items = append(o.items, item)
	o.known[path] = struct{}{}
	o.bytes += item.size
	o.signal()
	return nil
}

func (o *Outbox) Peek() (QueueItem, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.items) == 0 {
		return QueueItem{}, false
	}
	return QueueItem{path: o.items[0].path, size: o.items[0].size}, true
}

func (o *Outbox) Read(item QueueItem) ([]byte, error) {
	if item.path == "" {
		return nil, errors.New("queue item path is required")
	}
	data, err := os.ReadFile(item.path)
	if err != nil {
		return nil, fmt.Errorf("read queued batch: %w", err)
	}
	return data, nil
}

func (o *Outbox) Remove(item QueueItem) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	index := o.indexOf(item.path)
	if index < 0 {
		return nil
	}
	if err := os.Remove(item.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove uploaded batch: %w", err)
	}
	o.removeAt(index)
	return nil
}

func (o *Outbox) DeadLetter(item QueueItem, reason string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	index := o.indexOf(item.path)
	if index < 0 {
		return nil
	}
	relative, err := filepath.Rel(o.root, item.path)
	if err != nil {
		return fmt.Errorf("resolve dead-letter path: %w", err)
	}
	destination := filepath.Join(o.deadLetterDir, relative)
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("create dead-letter batch directory: %w", err)
	}
	if _, err := os.Stat(destination); err == nil {
		destination += "." + time.Now().UTC().Format("20060102T150405.000000000Z")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect dead-letter destination: %w", err)
	}
	if err := os.Rename(item.path, destination); err != nil {
		return fmt.Errorf("move batch to dead-letter directory: %w", err)
	}
	metadata := map[string]any{
		"failed_at": time.Now().UTC(),
		"reason":    reason,
	}
	if metadataData, marshalErr := json.MarshalIndent(metadata, "", "  "); marshalErr == nil {
		metadataData = append(metadataData, '\n')
		_ = atomicWrite(destination+".reason.json", metadataData)
	}
	o.removeAt(index)
	o.deadLetters++
	return nil
}

func (o *Outbox) Stats() QueueStats {
	o.mu.Lock()
	defer o.mu.Unlock()
	return QueueStats{
		Batches:     len(o.items),
		Bytes:       o.bytes,
		DeadLetters: o.deadLetters,
		MaxBatches:  o.maxBatches,
		MaxBytes:    o.maxBytes,
	}
}

func (o *Outbox) AtCapacity() bool {
	stats := o.Stats()
	return stats.Batches >= stats.MaxBatches || stats.Bytes >= stats.MaxBytes
}

func (o *Outbox) Wake() <-chan struct{} {
	return o.wake
}

func (o *Outbox) scan() error {
	var files []queuedFile
	err := filepath.WalkDir(o.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, queuedFile{path: path, size: info.Size(), modTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan outbox: %w", err)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].path < files[j].path
		}
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files {
		o.items = append(o.items, file)
		o.known[file.path] = struct{}{}
		o.bytes += file.size
	}
	deadLetters, err := countDeadLetters(o.deadLetterDir)
	if err != nil {
		return err
	}
	o.deadLetters = deadLetters
	return nil
}

func (o *Outbox) batchPath(batch schema.Batch) string {
	return filepath.Join(
		o.root,
		sanitisePathPart(batch.ServerID),
		sanitisePathPart(batch.ServerSessionID),
		fmt.Sprintf("%020d.json", batch.BatchSequence),
	)
}

func (o *Outbox) indexOf(path string) int {
	if _, ok := o.known[path]; !ok {
		return -1
	}
	for index := range o.items {
		if o.items[index].path == path {
			return index
		}
	}
	return -1
}

func (o *Outbox) removeAt(index int) {
	item := o.items[index]
	o.bytes -= item.size
	delete(o.known, item.path)
	copy(o.items[index:], o.items[index+1:])
	o.items = o.items[:len(o.items)-1]
}

func (o *Outbox) signal() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func sanitisePathPart(value string) string {
	cleaned := agentSafePathPart.ReplaceAllString(value, "_")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "unknown"
	}
	return cleaned
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create queued batch directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".batch-*.tmp")
	if err != nil {
		return fmt.Errorf("create queued batch temporary file: %w", err)
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
		return fmt.Errorf("set queued batch permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write queued batch: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync queued batch: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close queued batch: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("commit queued batch: %w", err)
	}
	removeTemp = false
	if runtime.GOOS != "windows" {
		directory, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("open queued batch directory: %w", err)
		}
		defer directory.Close()
		if err := directory.Sync(); err != nil {
			return fmt.Errorf("sync queued batch directory: %w", err)
		}
	}
	return nil
}

func countDeadLetters(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.Contains(filepath.Base(path), ".json") && !strings.HasSuffix(path, ".reason.json") {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("scan dead-letter directory: %w", err)
	}
	return count, nil
}
