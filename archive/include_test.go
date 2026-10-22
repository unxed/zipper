package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func archiveForIncludeTest(t *testing.T, name string) string {
	t.Helper()
	src := t.TempDir()
	files := map[string]os.FileInfo{}
	for rel, content := range map[string]string{
		"root.txt":                  "root",
		"selected/file.txt":         "selected",
		"selected/sub/deep.txt":     "deep",
		"selected-sibling/file.txt": "sibling",
		"other.txt":                 "other",
	} {
		filename := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatal(err)
		}
		files[filename] = info
	}
	archivePath := filepath.Join(t.TempDir(), name)
	a, err := NewArchiver(archivePath, src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), files); err != nil {
		_ = a.Close()
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func TestExtractorIncludeFilesAndDirectorySubtrees(t *testing.T) {
	for _, name := range []string{"sample.zip", "sample.tar", "sample.7z"} {
		t.Run(name, func(t *testing.T) {
			archivePath := archiveForIncludeTest(t, name)
			dst := t.TempDir()
			e, err := NewExtractor(archivePath, dst, Options{Include: []string{"root.txt", "selected/"}})
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			if err := e.Extract(context.Background()); err != nil {
				t.Fatal(err)
			}

			for rel, want := range map[string]string{
				"root.txt":              "root",
				"selected/file.txt":     "selected",
				"selected/sub/deep.txt": "deep",
			} {
				got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
				if err != nil {
					t.Errorf("selected path %q was not extracted: %v", rel, err)
					continue
				}
				if string(got) != want {
					t.Errorf("%s = %q, want %q", rel, got, want)
				}
			}
			for _, rel := range []string{"other.txt", "selected-sibling/file.txt"} {
				if _, err := os.Lstat(filepath.Join(dst, filepath.FromSlash(rel))); !os.IsNotExist(err) {
					t.Errorf("unselected path %q exists (stat error: %v)", rel, err)
				}
			}
		})
	}
}

func TestNewExtractorRejectsUnsafeIncludePaths(t *testing.T) {
	for _, name := range []string{"", ".", "../outside.txt", "/absolute.txt", `C:/outside.txt`, `dir\\file.txt`} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewExtractor("unused.zip", t.TempDir(), Options{Include: []string{name}}); err == nil {
				t.Fatalf("NewExtractor accepted unsafe include path %q", name)
			}
		})
	}
}
