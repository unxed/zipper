package archive

import (
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
