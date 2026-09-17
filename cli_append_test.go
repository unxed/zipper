//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestCli_AppendDirectory: "zipper a" takes a directory and adds what is under
// it. It used to open each target and copy it as a file, so a directory failed
// with "read ...: is a directory". A zip archive gets the directories, the
// empty one included, and the symbolic link as a link. A FIFO has no entry
// and is skipped rather than opened, which would block.
func TestCli_AppendDirectory(t *testing.T) {
	for _, ext := range []string{"zip"} {
		t.Run(ext, func(t *testing.T) {
			tmp := t.TempDir()
			mustWriteTestFile(t, filepath.Join(tmp, "file1.txt"), "one")
			mustWriteTestFile(t, filepath.Join(tmp, "adddir", "sub", "f2.txt"), "two")
			if err := os.MkdirAll(filepath.Join(tmp, "adddir", "empty"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("sub/f2.txt", filepath.Join(tmp, "adddir", "link")); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(filepath.Join(tmp, "adddir", "fifo"), 0o644); err != nil {
				t.Fatal(err)
			}

			arc := filepath.Join(tmp, "archive."+ext)
			if err := runZipper([]string{"zipper", "c", "-C", tmp, arc, "file1.txt"}); err != nil {
				t.Fatalf("c: %v", err)
			}
			if err := runZipper([]string{"zipper", "a", "-C", tmp, arc, "adddir"}); err != nil {
				t.Fatalf("a: %v", err)
			}

			dst := filepath.Join(tmp, "dst")
			if err := runZipper([]string{"zipper", "x", "-C", dst, arc}); err != nil {
				t.Fatalf("x: %v", err)
			}
			for name, want := range map[string]string{"file1.txt": "one", "adddir/sub/f2.txt": "two"} {
				b, err := os.ReadFile(filepath.Join(dst, name))
				if err != nil || string(b) != want {
					t.Errorf("%s: got %q, %v; want %q", name, b, err, want)
				}
			}
			if _, err := os.Lstat(filepath.Join(dst, "adddir", "fifo")); !os.IsNotExist(err) {
				t.Errorf("the FIFO was extracted (%v); it should have been skipped", err)
			}
			if fi, err := os.Stat(filepath.Join(dst, "adddir", "empty")); err != nil || !fi.IsDir() {
				t.Errorf("adddir/empty is not a directory: %v", err)
			}
			if target, err := os.Readlink(filepath.Join(dst, "adddir", "link")); err != nil || target != "sub/f2.txt" {
				t.Errorf("adddir/link: target %q, %v; want a link to sub/f2.txt", target, err)
			}
		})
	}
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
