package archive

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// buildIndexBackendTestArchive creates a small, real, uncompressed (non-solid,
// non-embedded-index) tar with one file, for the Options.IndexBackend tests
// below - a plain archive with no sidecar or embedded index of its own, so
// OpenFS always has to build one from scratch via whichever backend was
// requested.
func buildIndexBackendTestArchive(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(src, "hello.txt")
	if err := os.WriteFile(filePath, []byte("index backend data"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(tmp, "test.tar")
	a, err := NewArchiver(archivePath, src, Options{Method: "store", EmbeddedIdx: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), map[string]os.FileInfo{filePath: info}); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func readAllFromArchiveFS(t *testing.T, fsys FileSystem, name string) string {
	t.Helper()
	f, err := fsys.Open(name)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("reading %q failed: %v", name, err)
	}
	return string(data)
}

// TestOpenFSIndexBackendArcidx verifies OpenFS(..., Options{IndexBackend:
// IndexBackendArcidx}) opens and reads a tar archive through unxed/tar's
// FlatBuffers-backed index, without needing zipper (or unxed/tar) to be
// built with any special tag.
func TestOpenFSIndexBackendArcidx(t *testing.T) {
	archivePath := buildIndexBackendTestArchive(t)

	fsys, err := OpenFS(archivePath, Options{IndexBackend: IndexBackendArcidx})
	if err != nil {
		t.Fatalf("OpenFS with IndexBackendArcidx failed: %v", err)
	}
	defer fsys.Close()

	if got := readAllFromArchiveFS(t, fsys, "hello.txt"); got != "index backend data" {
		t.Fatalf("got %q, want %q", got, "index backend data")
	}
}

// TestOpenFSIndexBackendSQLite is the IndexBackendArcidx test's mirror for
// the sqlite-backed index, explicitly requested rather than left to the
// default.
func TestOpenFSIndexBackendSQLite(t *testing.T) {
	archivePath := buildIndexBackendTestArchive(t)

	fsys, err := OpenFS(archivePath, Options{IndexBackend: IndexBackendSQLite})
	if err != nil {
		t.Fatalf("OpenFS with IndexBackendSQLite failed: %v", err)
	}
	defer fsys.Close()

	if got := readAllFromArchiveFS(t, fsys, "hello.txt"); got != "index backend data" {
		t.Fatalf("got %q, want %q", got, "index backend data")
	}
}

// TestOpenFSIndexBackendAutoUnchanged pins down that the zero value of the
// new field (IndexBackendAuto, i.e. "") keeps opening successfully, exactly
// as every existing caller that never sets IndexBackend already relies on.
func TestOpenFSIndexBackendAutoUnchanged(t *testing.T) {
	archivePath := buildIndexBackendTestArchive(t)

	fsys, err := OpenFS(archivePath, Options{})
	if err != nil {
		t.Fatalf("OpenFS with the default IndexBackendAuto failed: %v", err)
	}
	defer fsys.Close()

	if got := readAllFromArchiveFS(t, fsys, "hello.txt"); got != "index backend data" {
		t.Fatalf("got %q, want %q", got, "index backend data")
	}
}

// TestOpenFSIndexBackendInvalid verifies an unrecognized IndexBackend value
// is a clear error rather than silently falling back to the default.
func TestOpenFSIndexBackendInvalid(t *testing.T) {
	archivePath := buildIndexBackendTestArchive(t)

	_, err := OpenFS(archivePath, Options{IndexBackend: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an unrecognized IndexBackend value, got nil")
	}
}
