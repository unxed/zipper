package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// captureStderr redirects os.Stderr to a pipe for the duration of fn and
// returns everything written to it, restoring os.Stderr before returning.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	w.Close()
	os.Stderr = oldStderr
	return <-done
}

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

// TestCli_CreateEachFileSkipsFileDeletedDuringWalk exercises the same race as
// TestCli_CreateSkipsFileDeletedDuringWalk, but with "-eachfile", where every
// walked file gets its own archive. Losing one file to a race must not stop
// the others from getting archived, and the half-written archive for the
// vanished file must be cleaned up rather than left behind empty or corrupt.
func TestCli_CreateEachFileSkipsFileDeletedDuringWalk(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("kept content"), 0644); err != nil {
		t.Fatalf("failed to write keep.txt: %v", err)
	}
	deletedPath := filepath.Join(srcDir, "deleteme.txt")
	if err := os.WriteFile(deletedPath, []byte("will vanish"), 0644); err != nil {
		t.Fatalf("failed to write deleteme.txt: %v", err)
	}

	testHookFilesCollected = func(collected map[string]os.FileInfo) {
		if _, ok := collected[deletedPath]; !ok {
			t.Errorf("walk did not collect %s before it was deleted", deletedPath)
		}
		if err := os.Remove(deletedPath); err != nil {
			t.Fatalf("failed to delete %s during walk/archive gap: %v", deletedPath, err)
		}
	}
	defer func() { testHookFilesCollected = nil }()

	archivePath := filepath.Join(tmp, "out.zip")

	var runErr error
	stderrOutput := captureStderr(t, func() {
		runErr = runZipper([]string{"zipper", "c", "-C", srcDir, "-eachfile", archivePath, "."})
	})

	if runErr != nil {
		t.Fatalf("runZipper(c -eachfile) failed even though only a deleted file was missing: %v", runErr)
	}

	lower := strings.ToLower(stderrOutput)
	if !strings.Contains(stderrOutput, "deleteme.txt") || !strings.Contains(lower, "warning") {
		t.Errorf("expected a warning about deleteme.txt on stderr, got: %q", stderrOutput)
	}

	outDirTarget := filepath.Join(tmp, "out")
	if _, err := os.Stat(filepath.Join(outDirTarget, "keep.txt.zip")); err != nil {
		t.Errorf("expected keep.txt.zip to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDirTarget, "deleteme.txt.zip")); !os.IsNotExist(err) {
		t.Errorf("expected deleteme.txt.zip to have been cleaned up, stat err: %v", err)
	}
}

// TestCli_CreateFailsFatallyOnNonMissingFileError makes sure the retry/skip
// logic added for the deleted-during-walk race does not swallow unrelated
// archiving errors. A file that the archiver cannot open for a reason other
// than it no longer existing (here: permission denied) must still fail the
// whole "c" command, exactly as before that logic was added.
func TestCli_CreateFailsFatallyOnNonMissingFileError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permission bits do not restrict reads")
	}

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("kept content"), 0644); err != nil {
		t.Fatalf("failed to write keep.txt: %v", err)
	}
	lockedPath := filepath.Join(srcDir, "locked.txt")
	if err := os.WriteFile(lockedPath, []byte("cannot be read"), 0644); err != nil {
		t.Fatalf("failed to write locked.txt: %v", err)
	}
	defer os.Chmod(lockedPath, 0644)

	testHookFilesCollected = func(collected map[string]os.FileInfo) {
		if _, ok := collected[lockedPath]; !ok {
			t.Errorf("walk did not collect %s before its permissions changed", lockedPath)
		}
		// Simulate the file becoming unreadable (not deleted) between the
		// walk seeing it and the archiver opening it -- a permission error,
		// which the skip logic must NOT treat like a missing file.
		if err := os.Chmod(lockedPath, 0000); err != nil {
			t.Fatalf("failed to chmod %s: %v", lockedPath, err)
		}
	}
	defer func() { testHookFilesCollected = nil }()

	archivePath := filepath.Join(tmp, "out.zip")

	var runErr error
	stderrOutput := captureStderr(t, func() {
		runErr = runZipper([]string{"zipper", "c", "-C", srcDir, archivePath, "."})
	})

	if runErr == nil {
		t.Fatal("expected runZipper(c) to fail fatally on a permission error, got nil")
	}
	if errors.Is(runErr, os.ErrNotExist) {
		t.Errorf("permission error was mistaken for a missing file: %v", runErr)
	}
	if strings.Contains(strings.ToLower(stderrOutput), "warning") {
		t.Errorf("did not expect a skip warning for a fatal error, got: %q", stderrOutput)
	}
}

// TestCli_AppendSkipsFileDeletedBeforeOpen reproduces the same race as
// TestCli_CreateSkipsFileDeletedDuringWalk, but for "zipper a": a file is
// deleted by another process after appendTargets' walk has seen it but
// before AppendFile opens it for reading. It should be skipped with a
// warning, like the "c" command, rather than aborting the whole append.
func TestCli_AppendSkipsFileDeletedBeforeOpen(t *testing.T) {
	tmp := t.TempDir()

	baseDir := filepath.Join(tmp, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("failed to create base dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "orig.txt"), []byte("original content"), 0644); err != nil {
		t.Fatalf("failed to write orig.txt: %v", err)
	}

	archivePath := filepath.Join(tmp, "archive.zip")
	if err := runZipper([]string{"zipper", "c", "-C", baseDir, archivePath, "."}); err != nil {
		t.Fatalf("failed to create base archive: %v", err)
	}

	appendDir := filepath.Join(tmp, "append")
	if err := os.MkdirAll(appendDir, 0755); err != nil {
		t.Fatalf("failed to create append dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appendDir, "keep2.txt"), []byte("kept content"), 0644); err != nil {
		t.Fatalf("failed to write keep2.txt: %v", err)
	}
	vanishPath := filepath.Join(appendDir, "vanish.txt")
	if err := os.WriteFile(vanishPath, []byte("will vanish before append"), 0644); err != nil {
		t.Fatalf("failed to write vanish.txt: %v", err)
	}

	var hookSawVanish bool
	testHookBeforeAppendFile = func(p string) {
		if p != vanishPath {
			return
		}
		hookSawVanish = true
		if err := os.Remove(vanishPath); err != nil {
			t.Fatalf("failed to delete %s before append: %v", vanishPath, err)
		}
	}
	defer func() { testHookBeforeAppendFile = nil }()

	var runErr error
	stderrOutput := captureStderr(t, func() {
		runErr = runZipper([]string{"zipper", "a", "-C", appendDir, archivePath, "."})
	})

	if runErr != nil {
		t.Fatalf("runZipper(a) failed even though only a deleted file was missing: %v", runErr)
	}
	if !hookSawVanish {
		t.Fatal("expected the append walk to reach vanish.txt before it was deleted")
	}

	lower := strings.ToLower(stderrOutput)
	if !strings.Contains(stderrOutput, "vanish.txt") || !strings.Contains(lower, "warning") {
		t.Errorf("expected a warning about vanish.txt on stderr, got: %q", stderrOutput)
	}

	dstDir := filepath.Join(tmp, "dst")
	if err := runZipper([]string{"zipper", "x", "-C", dstDir, archivePath}); err != nil {
		t.Fatalf("runZipper(x) failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dstDir, "vanish.txt")); !os.IsNotExist(err) {
		t.Errorf("expected vanish.txt to be absent from the extracted archive, stat err: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dstDir, "orig.txt")); err != nil || string(got) != "original content" {
		t.Errorf("expected orig.txt to survive the append intact, got %q, err %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dstDir, "keep2.txt")); err != nil || string(got) != "kept content" {
		t.Errorf("expected keep2.txt to have been appended, got %q, err %v", got, err)
	}
}
