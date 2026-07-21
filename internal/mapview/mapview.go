package mapview

import (
	"context"
	"errors"
	"sync"
)

type Map struct {
	Name          string `json:"name"`
	Size          int    `json:"size"`
	Zoom          int    `json:"zoom"`
	NoTopographic bool   `json:"no_topographic,omitempty"`
	NoSatellite   bool   `json:"no_satellite,omitempty"`
	Attribution   string `json:"attribution,omitempty"`
}

type TileRequest struct {
	MapName string
	Layer   string
	Zoom    int
	X       int
	Y       int
}

type Tile struct {
	Contents    []byte
	ContentType string
	ETag        string
}

type Source interface {
	Maps(context.Context) ([]Map, error)
	Tile(context.Context, TileRequest) (Tile, error)
}

type MemorySource struct {
	MapValues  []Map
	TileValues map[TileRequest]Tile
	mu         sync.RWMutex
}

func (s *MemorySource) Maps(context.Context) ([]Map, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Map(nil), s.MapValues...), nil
}

func (s *MemorySource) Tile(_ context.Context, request TileRequest) (Tile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.TileValues[request]
	if !ok {
		return Tile{}, ErrNotFound
	}
	value.Contents = append([]byte(nil), value.Contents...)
	return value, nil
}

var ErrNotFound = errors.New("map resource not found")

var _ Source = (*MemorySource)(nil)
