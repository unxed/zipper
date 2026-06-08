package archive

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

func TestZipEngine_Encryption(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "secret.txt"), []byte("super secret data"), 0644)

	arc := filepath.Join(tmp, "enc.zip")
	opts := Options{
		Password:  "12345",
		EncryptCD: true,
	}
	a, err := NewArchiver(arc, src, opts)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(src, "secret.txt"))
	a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "secret.txt"): info})
	a.Close()

	os.MkdirAll(dst, 0755)
	e, err := NewExtractor(arc, dst, opts)
	if err != nil {
		t.Fatal(err)
	}
	err = e.Extract(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e.Close()

	b, _ := os.ReadFile(filepath.Join(dst, "secret.txt"))
	if string(b) != "super secret data" {
		t.Errorf("content mismatch")
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
func TestTarEngine_Encryption(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "secret.txt"), []byte("tar super secret data"), 0644)

	arc := filepath.Join(tmp, "enc.tar.zst")
	opts := Options{
		Password: "tar_password",
	}
	a, err := NewArchiver(arc, src, opts)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(src, "secret.txt"))
	a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "secret.txt"): info})
	a.Close()

	os.MkdirAll(dst, 0755)
	e, err := NewExtractor(arc, dst, opts)
	if err != nil {
		t.Fatal(err)
	}
	err = e.Extract(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e.Close()

	b, _ := os.ReadFile(filepath.Join(dst, "secret.txt"))
	if string(b) != "tar super secret data" {
		t.Errorf("content mismatch")
	}
}

func TestTarEngine_EmbeddedIndex(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("tar embedded index data"), 0644)

	arc := filepath.Join(tmp, "test_embedded.tar.zst")
	idx := filepath.Join(tmp, "index.sqlite")
	a, err := NewArchiver(arc, src, Options{
		Method:      "zstd",
		IndexPath:   idx,
		EmbeddedIdx: true,
		Xattrs:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(src, "test.txt"))
	err = a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "test.txt"): info})
	if err != nil {
		t.Fatal(err)
	}
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
	if string(b) != "tar embedded index data" {
		t.Errorf("expected 'tar embedded index data', got %q", string(b))
	}
}