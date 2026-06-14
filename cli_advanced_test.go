package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCli_Excludes(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(srcDir, 0755)

	os.WriteFile(filepath.Join(srcDir, "include.bin"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(srcDir, "exclude.txt"), []byte("data"), 0644)

	archivePath := filepath.Join(tmp, "exclude_test.zip")

	err := runZipper([]string{"zipper", "c", "-C", srcDir, "-exclude", "*.txt", archivePath, "."})
	if err != nil {
		t.Fatalf("zipper c with excludes failed: %v", err)
	}

	os.MkdirAll(dstDir, 0755)
	err = runZipper([]string{"zipper", "x", "-C", dstDir, archivePath})
	if err != nil {
		t.Fatalf("zipper x failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dstDir, "exclude.txt")); err == nil {
		t.Error("expected exclude.txt to be excluded from zip archive, but it was found")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "include.bin")); err != nil {
		t.Error("expected include.bin to be included in zip archive, but it was missing")
	}

	os.RemoveAll(dstDir)
	os.Remove(archivePath)
	tarPath := filepath.Join(tmp, "exclude_test.tar")

	oldWd, _ := os.Getwd()
	os.Chdir(srcDir)
	defer os.Chdir(oldWd)

	err = runTar([]string{"tar", "-cf", tarPath, "--exclude=*.txt", "."})
	if err != nil {
		t.Fatalf("tar cf with excludes failed: %v", err)
	}

	os.Chdir(oldWd)
	os.MkdirAll(dstDir, 0755)
	os.Chdir(dstDir)
	err = runTar([]string{"tar", "-xf", tarPath})
	os.Chdir(oldWd)
	if err != nil {
		t.Fatalf("tar xf failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dstDir, "exclude.txt")); err == nil {
		t.Error("expected exclude.txt to be excluded from tar, but it was found")
	}
}

func TestCli_ListAndProgress(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("list data"), 0644)

	archivePath := filepath.Join(tmp, "list_progress.zip")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	err := runZipper([]string{"zipper", "c", "-C", srcDir, "-progress", archivePath, "test.txt"})
	if err != nil {
		t.Fatalf("zipper create with progress failed: %v", err)
	}

	err = runZipper([]string{"zipper", "l", archivePath})
	if err != nil {
		t.Fatalf("zipper list failed: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "test.txt") {
		t.Errorf("expected list output to contain 'test.txt', got:\n%s", output)
	}
	if !strings.Contains(output, "Length") || !strings.Contains(output, "Date   Time") {
		t.Errorf("expected list header, got:\n%s", output)
	}
}

func TestCli_ZipExcludes(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "include.bin"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(srcDir, "exclude.txt"), []byte("data"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(srcDir)
	defer os.Chdir(oldWd)

	archivePath := filepath.Join(tmp, "zip_exclude.zip")
	err := runZip([]string{"zip", "-r", "-x", "*.txt", archivePath, "."})
	if err != nil {
		t.Fatalf("zip with excludes failed: %v", err)
	}

	os.Chdir(oldWd)
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(dstDir, 0755)
	err = runUnzip([]string{"unzip", archivePath, "-d", dstDir})
	if err != nil {
		t.Fatalf("unzip failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dstDir, "exclude.txt")); err == nil {
		t.Error("expected exclude.txt to be excluded by zip -x")
	}
}