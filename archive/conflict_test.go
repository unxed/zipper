package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractorConflictSkip(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	dst := filepath.Join(tmp, "destination")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("archive contents"), 0644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(tmp, "files.zip")
	a, err := NewArchiver(archivePath, src, Options{Method: "store"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(src, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "file.txt"): info}); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dst, "file.txt")
	if err := os.WriteFile(target, []byte("existing contents"), 0644); err != nil {
		t.Fatal(err)
	}
	var conflicts []string
	e, err := NewExtractor(archivePath, dst, Options{OnConflict: func(name string) ConflictAction {
		conflicts = append(conflicts, name)
		return ConflictSkip
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Extract(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || conflicts[0] != "file.txt" {
		t.Fatalf("conflict callback calls = %v, want [file.txt]", conflicts)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing contents" {
		t.Fatalf("skipped target contents = %q, want existing contents", got)
	}
}
