package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestZipEngine(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("zip data"), 0644)

	arc := filepath.Join(tmp, "test.zip")
	a, err := NewArchiver(arc, src, Options{Method: "deflate", Xattrs: true})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(src, "test.txt"))
	a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "test.txt"): info})
	a.Close()

	os.MkdirAll(dst, 0755)
	e, err := NewExtractor(arc, dst, Options{Xattrs: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Extract(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.Close()

	b, _ := os.ReadFile(filepath.Join(dst, "test.txt"))
	if string(b) != "zip data" {
		t.Errorf("expected 'zip data', got %q", string(b))
	}
}

func TestTarEngine(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("tar data"), 0644)

	arc := filepath.Join(tmp, "test.tar.zst")
	a, err := NewArchiver(arc, src, Options{Xattrs: true})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(src, "test.txt"))
	a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "test.txt"): info})
	a.Close()

	os.MkdirAll(dst, 0755)
	e, err := NewExtractor(arc, dst, Options{Xattrs: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Extract(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.Close()

	b, _ := os.ReadFile(filepath.Join(dst, "test.txt"))
	if string(b) != "tar data" {
		t.Errorf("expected 'tar data', got %q", string(b))
	}
}