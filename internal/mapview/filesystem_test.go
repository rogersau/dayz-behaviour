package mapview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemLoadsCatalogAndTile(t *testing.T) {
	root := t.TempDir()
	tileDirectory := filepath.Join(root, "namalsk", "topographic", "1", "0")
	if err := os.MkdirAll(tileDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(`[{"name":"namalsk","size":12800,"zoom":4,"attribution":"Tiles © Xam.nu"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tileDirectory, "1.webp"), []byte("webp"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := OpenFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	maps, err := source.Maps(context.Background())
	if err != nil || len(maps) != 1 || maps[0].Name != "namalsk" {
		t.Fatalf("maps = %#v, error = %v", maps, err)
	}
	tile, err := source.Tile(context.Background(), TileRequest{MapName: "namalsk", Layer: "topographic", Zoom: 1, X: 0, Y: 1})
	if err != nil || string(tile.Contents) != "webp" || tile.ContentType != "image/webp" || tile.ETag == "" {
		t.Fatalf("tile = %#v, error = %v", tile, err)
	}
	if _, err := source.Tile(context.Background(), TileRequest{MapName: "../bad", Layer: "topographic"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unsafe tile error = %v", err)
	}
}

func TestFilesystemRejectsMissingAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(`[{"name":"namalsk","size":12800,"zoom":4}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFilesystem(root); err == nil {
		t.Fatal("expected missing assets error")
	}
}
