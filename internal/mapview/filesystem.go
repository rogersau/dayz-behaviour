package mapview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
)

const maxTileBytes = 2 * 1024 * 1024

var validName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Filesystem struct {
	root   string
	maps   []Map
	byName map[string]Map
	mu     sync.RWMutex
}

func OpenFilesystem(root string) (*Filesystem, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve map directory: %w", err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil || !info.IsDir() {
		return nil, errors.New("map directory does not exist")
	}
	catalog, err := os.Open(filepath.Join(absoluteRoot, "catalog.json"))
	if err != nil {
		return nil, fmt.Errorf("open map catalog: %w", err)
	}
	defer catalog.Close()
	var values []Map
	if err := json.NewDecoder(io.LimitReader(catalog, 256*1024)).Decode(&values); err != nil {
		return nil, fmt.Errorf("decode map catalog: %w", err)
	}
	byName := make(map[string]Map, len(values))
	result := make([]Map, 0, len(values))
	for _, value := range values {
		if !validName.MatchString(value.Name) || value.Size <= 0 || value.Zoom < 0 || value.Zoom > 20 {
			return nil, fmt.Errorf("invalid map catalog entry %q", value.Name)
		}
		if _, duplicate := byName[value.Name]; duplicate {
			return nil, fmt.Errorf("duplicate map catalog entry %q", value.Name)
		}
		if info, err := os.Stat(filepath.Join(absoluteRoot, value.Name)); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("map assets missing for %q", value.Name)
		}
		byName[value.Name] = value
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, errors.New("map catalog is empty")
	}
	return &Filesystem{root: absoluteRoot, maps: result, byName: byName}, nil
}

func (f *Filesystem) Maps(context.Context) ([]Map, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]Map(nil), f.maps...), nil
}

func (f *Filesystem) Tile(ctx context.Context, request TileRequest) (Tile, error) {
	if err := ctx.Err(); err != nil {
		return Tile{}, err
	}
	if !validName.MatchString(request.MapName) || (request.Layer != "topographic" && request.Layer != "satellite") || request.Zoom < 0 {
		return Tile{}, ErrNotFound
	}
	f.mu.RLock()
	mapValue, exists := f.byName[request.MapName]
	f.mu.RUnlock()
	if !exists || request.Zoom > mapValue.Zoom {
		return Tile{}, ErrNotFound
	}
	maximum := 1 << request.Zoom
	if request.X < 0 || request.Y < 0 || request.X >= maximum || request.Y >= maximum {
		return Tile{}, ErrNotFound
	}
	path := filepath.Join(f.root, request.MapName, request.Layer, strconv.Itoa(request.Zoom), strconv.Itoa(request.X), strconv.Itoa(request.Y)+".webp")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return Tile{}, ErrNotFound
	}
	if err != nil {
		return Tile{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxTileBytes {
		return Tile{}, errors.New("invalid map tile")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return Tile{}, err
	}
	etag := `"` + strconv.FormatInt(info.Size(), 16) + `-` + strconv.FormatInt(info.ModTime().UnixNano(), 16) + `"`
	return Tile{Contents: contents, ContentType: "image/webp", ETag: etag}, nil
}

var _ Source = (*Filesystem)(nil)
