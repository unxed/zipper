package archive

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useTempCache points os.UserCacheDir at a directory of the test on every OS
// and returns the directory the tar indexes go to.
func useTempCache(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("HOME", home)
	t.Setenv("LocalAppData", filepath.Join(home, "cache"))
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(cacheDir, "f4", "tar-indexes")
}

func countFiles(t *testing.T, filename string) int {
	t.Helper()
	fsys, err := OpenFS(filename, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer fsys.Close()
	n := 0
	err = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestOpenFSListsTarAppendedInPlace: the index OpenFS caches for a tar was
// once reused after NewUpdater appended to the archive, and the new entry was
// missing from the listing (unxed/zipper#35, Z7).
func TestOpenFSListsTarAppendedInPlace(t *testing.T) {
	indexDir := useTempCache(t)
	for _, name := range []string{"x.tar", "x.tar.zst", "x.tar.gz", "x.tar.xz"} {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			files := map[string]os.FileInfo{}
			for _, f := range []string{"a.txt", "b.txt"} {
				p := filepath.Join(src, f)
				if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
					t.Fatal(err)
				}
				info, err := os.Lstat(p)
				if err != nil {
					t.Fatal(err)
				}
				files[p] = info
			}
			arc := filepath.Join(t.TempDir(), name)
			a, err := NewArchiver(arc, src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Archive(context.Background(), files); err != nil {
				a.Close()
				t.Fatal(err)
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if n := countFiles(t, arc); n != 2 {
				t.Fatalf("new %s lists %d files, want 2", name, n)
			}

			u, err := NewUpdater(arc, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if err := u.Append("c.txt", 3, strings.NewReader("abc")); err != nil {
				u.Close()
				t.Fatal(err)
			}
			if err := u.Close(); err != nil {
				t.Fatal(err)
			}
			if n := countFiles(t, arc); n != 3 {
				t.Fatalf("%s after the append lists %d files, want 3", name, n)
			}

			// The index of the archive as it was is gone; one remains.
			entries, err := os.ReadDir(indexDir)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), name+"-") && strings.HasSuffix(e.Name(), ".index.sqlite") {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("%d cached indexes of %s, want 1", n, name)
			}
		})
	}
}

func TestIndexPathFollowsArchiveContent(t *testing.T) {
	useTempCache(t)
	arc := filepath.Join(t.TempDir(), "x.tar")
	if err := os.WriteFile(arc, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := getDeterministicIndexPath(arc)
	if again := getDeterministicIndexPath(arc); again != first {
		t.Fatalf("same archive, two index paths: %q and %q", first, again)
	}
	if err := os.WriteFile(arc, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed := getDeterministicIndexPath(arc); changed == first {
		t.Fatalf("archive changed, index path stayed %q", first)
	}
}
