package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestCli_CreateSkipsFileDeletedDuringWalk reproduces the race reported in
// https://github.com/unxed/zipper/issues/19: an external process deletes a
// file after the directory walk for "zipper c" has already seen it, but
// before the archiver gets around to opening it. Before the fix this made
// the whole "c" command fail with a fatal "no such file or directory"
// error, discarding every other file it had collected. It should instead
// skip the vanished file with a warning on stderr and archive everything
// else, the way tar and zip do.
//
// The deletion is injected through testHookFilesCollected -- a hook that
// runs right after the walk has built its file list and before archiving
// starts -- rather than relying on a timing-dependent race between a
// background goroutine and tiny, near-instant test files.
func TestCli_CreateSkipsFileDeletedDuringWalk(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	files := map[string]string{
		"file1.txt":     "first file content",
		"file2.txt":     "this file will disappear",
		"sub/file3.txt": "third file content, in a subdirectory",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	deletedPath := filepath.Join(srcDir, "file2.txt")

	var hookCalls int
	var mu sync.Mutex
	testHookFilesCollected = func(collected map[string]os.FileInfo) {
		mu.Lock()
		hookCalls++
		mu.Unlock()

		// Sanity check: the walk really did see the file we are about to
		// delete, otherwise this test would not exercise the race at all.
		if _, ok := collected[deletedPath]; !ok {
			t.Errorf("walk did not collect %s before it was deleted", deletedPath)
		}

		// Simulate another process removing the file in the gap between
		// the walk seeing it and the archiver opening it.
		if err := os.Remove(deletedPath); err != nil {
			t.Fatalf("failed to delete %s during walk/archive gap: %v", deletedPath, err)
		}
	}
	defer func() { testHookFilesCollected = nil }()

	archivePath := filepath.Join(tmp, "out.zip")

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	stderrDone := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		stderrDone <- string(b)
	}()

	runErr := runZipper([]string{"zipper", "c", "-C", srcDir, archivePath, "."})

	w.Close()
	os.Stderr = oldStderr
	stderrOutput := <-stderrDone

	if runErr != nil {
		t.Fatalf("runZipper(c) failed even though only a deleted file was missing: %v", runErr)
	}

	mu.Lock()
	calls := hookCalls
	mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected the walk hook to run exactly once, ran %d times", calls)
	}

	if !strings.Contains(stderrOutput, "file2.txt") {
		t.Errorf("expected a warning about file2.txt on stderr, got: %q", stderrOutput)
	}
	lower := strings.ToLower(stderrOutput)
	if !strings.Contains(lower, "warning") {
		t.Errorf("expected the message to be a warning, got: %q", stderrOutput)
	}

	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive %s to exist: %v", archivePath, err)
	}

	dstDir := filepath.Join(tmp, "dst")
	if err := runZipper([]string{"zipper", "x", "-C", dstDir, archivePath}); err != nil {
		t.Fatalf("runZipper(x) failed: %v", err)
	}

	// The deleted file must not be in the archive at all.
	if _, err := os.Stat(filepath.Join(dstDir, "file2.txt")); !os.IsNotExist(err) {
		t.Errorf("expected file2.txt to be absent from the extracted archive, stat err: %v", err)
	}

	// Everything else the walk saw before the deletion must have made it
	// into the archive intact.
	for _, name := range []string{"file1.txt", filepath.Join("sub", "file3.txt")} {
		want := files[filepath.ToSlash(name)]
		got, err := os.ReadFile(filepath.Join(dstDir, name))
		if err != nil {
			t.Errorf("expected %s to be extracted: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s content mismatch: got %q, want %q", name, string(got), want)
		}
	}
}
