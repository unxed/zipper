package archive

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDeterministicIndexPath(t *testing.T) {
	archivePath := "/home/user/Downloads/test.tar.zst"
	indexPath := getDeterministicIndexPath(archivePath)

	if indexPath == "" {
		t.Fatal("Expected non-empty index path")
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	expectedSubdir := filepath.Join(cacheDir, "f4", "tar-indexes")

	if !strings.HasPrefix(indexPath, expectedSubdir) {
		t.Errorf("Expected index path to be in %q, got %q", expectedSubdir, indexPath)
	}

	if !strings.HasSuffix(indexPath, ".index.sqlite") {
		t.Errorf("Expected index path to end with '.index.sqlite', got %q", indexPath)
	}

	// Test determinism (same input must yield same output)
	indexPath2 := getDeterministicIndexPath(archivePath)
	if indexPath != indexPath2 {
		t.Error("getDeterministicIndexPath is not deterministic")
	}

	// Test uniqueness (different inputs must yield different outputs)
	differentPath := "/home/user/Downloads/other.tar.zst"
	indexPath3 := getDeterministicIndexPath(differentPath)
	if indexPath == indexPath3 {
		t.Error("Expected different index paths for different archives")
	}
}

func TestOpenFSFallbackWithPassword(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "secret.txt")
	archivePath := filepath.Join(tmp, "secret.7z")
	if err := os.WriteFile(src, []byte("secret data"), 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	archiver, err := NewArchiver(archivePath, tmp, Options{Password: "correct"})
	if err != nil {
		t.Fatal(err)
	}
	if err := archiver.Archive(context.Background(), map[string]os.FileInfo{src: info}); err != nil {
		t.Fatal(err)
	}
	if err := archiver.Close(); err != nil {
		t.Fatal(err)
	}

	fsys, err := OpenFS(archivePath, Options{Password: "correct"})
	if err != nil {
		t.Fatal(err)
	}
	defer fsys.Close()

	file, err := fsys.Open("secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret data" {
		t.Fatalf("got %q, want %q", data, "secret data")
	}
}
