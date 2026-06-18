package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unxed/zip"
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

func TestCli_Piping(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(srcDir, 0755)

	os.WriteFile(filepath.Join(srcDir, "pipe.txt"), []byte("piped data stream"), 0644)

	// 1. Archive to STDOUT
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldWd, _ := os.Getwd()
	os.Chdir(srcDir)
	err := runTar([]string{"tar", "-cf", "-", "pipe.txt"})
	os.Chdir(oldWd)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("tar cf to stdout failed: %v", err)
	}

	var archiveData bytes.Buffer
	io.Copy(&archiveData, r)

	if archiveData.Len() == 0 {
		t.Fatal("captured stdout archive data is empty")
	}

	// 2. Extract from STDIN
	os.MkdirAll(dstDir, 0755)
	oldStdin := os.Stdin
	inR, inW, _ := os.Pipe()
	os.Stdin = inR
	defer func() { os.Stdin = oldStdin }()

	go func() {
		inW.Write(archiveData.Bytes())
		inW.Close()
	}()

	os.Chdir(dstDir)
	err = runTar([]string{"tar", "-xf", "-"})
	os.Chdir(oldWd)
	if err != nil {
		t.Fatalf("tar xf from stdin failed: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dstDir, "pipe.txt"))
	if err != nil {
		t.Fatalf("failed to read piped file: %v", err)
	}
	if string(b) != "piped data stream" {
		t.Errorf("content mismatch: got %q, want 'piped data stream'", string(b))
	}
}

func TestCli_PipingZip(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(srcDir, 0755)

	os.WriteFile(filepath.Join(srcDir, "pipe.txt"), []byte("zip piped data"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(srcDir)
	defer os.Chdir(oldWd)

	// 1. Zip to STDOUT
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runZip([]string{"zip", "-r", "-", "pipe.txt"})
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("zip to stdout failed: %v", err)
	}

	var archiveData bytes.Buffer
	io.Copy(&archiveData, r)

	if archiveData.Len() == 0 {
		t.Fatal("captured zip stdout is empty")
	}

	// 2. Unzip from STDIN
	os.MkdirAll(dstDir, 0755)
	oldStdin := os.Stdin
	inR, inW, _ := os.Pipe()
	os.Stdin = inR
	defer func() { os.Stdin = oldStdin }()

	go func() {
		inW.Write(archiveData.Bytes())
		inW.Close()
	}()

	err = runUnzip([]string{"unzip", "-", "-d", dstDir})
	if err != nil {
		t.Fatalf("unzip from stdin failed: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dstDir, "pipe.txt"))
	if err != nil {
		t.Fatalf("failed to read unzipped file: %v", err)
	}
	if string(b) != "zip piped data" {
		t.Errorf("content mismatch: got %q, want 'zip piped data'", string(b))
	}
}
func TestCli_OutsideChrootArchiving(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	outsideDir := filepath.Join(tmp, "outside_data")
	os.MkdirAll(workspace, 0755)
	os.MkdirAll(outsideDir, 0755)

	outsideFile := filepath.Join(outsideDir, "test.txt")
	os.WriteFile(outsideFile, []byte("outside chroot content"), 0644)

	archivePath := filepath.Join(workspace, "archive.zip")

	err := runZipper([]string{"zipper", "c", "-C", workspace, archivePath, outsideFile})
	if err != nil {
		t.Fatalf("Expected success archiving outside file, got error: %v", err)
	}

	dstDir := filepath.Join(tmp, "extracted")
	os.MkdirAll(dstDir, 0755)
	err = runZipper([]string{"zipper", "x", "-C", dstDir, archivePath})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	found := false
	filepath.Walk(dstDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "test.txt" {
			b, _ := os.ReadFile(path)
			if string(b) == "outside chroot content" {
				found = true
			}
		}
		return nil
	})

	if !found {
		t.Error("Failed to find or verify the content of the safely normalized outside-chroot file")
	}
}

func TestCli_TorrentZipCRC32Validation(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("content a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("content b"), 0644)

	archivePath := filepath.Join(tmp, "torrent.zip")

	err := runZipper([]string{"zipper", "c", "-C", srcDir, "-torrentzip", archivePath, "a.txt", "sub/b.txt"})
	if err != nil {
		t.Fatalf("torrentzip creation failed: %v", err)
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open zip reader: %v", err)
	}
	defer zr.Close()

	if len(zr.File) < 2 {
		t.Fatalf("expected at least 2 files, got %d", len(zr.File))
	}

	for _, file := range zr.File {
		if file.CRC32 == 0 {
			t.Errorf("file %s has CRC32 = 00000000, torrentzip requires non-zero checksums", file.Name)
		}
		if file.Flags&0x8 != 0 {
			t.Errorf("file %s has Data Descriptor bit set, torrentzip forbids descriptors", file.Name)
		}
	}
}

func TestCli_TrimParents(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "downloads", "bass24", "delphi", "3dTest")
	os.MkdirAll(srcDir, 0755)

	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("data1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("data2"), 0644)

	archivePath := filepath.Join(tmp, "trim_parents_test.zip")

	// 1. Тест создания архива с опцией -trim-parents
	err := runZipper([]string{"zipper", "c", "-trim-parents", archivePath, srcDir})
	if err != nil {
		t.Fatalf("zipper c with -trim-parents failed: %v", err)
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}

	found3dTest := false
	foundFile1 := false
	for _, f := range zr.File {
		t.Logf("Zip entry name: %s", f.Name)
		if f.Name == "3dTest/" {
			found3dTest = true
		}
		if f.Name == "3dTest/file1.txt" {
			foundFile1 = true
		}
		if strings.Contains(f.Name, "downloads") {
			t.Errorf("found 'downloads' in path %q, expected it to be stripped", f.Name)
		}
	}
	zr.Close()

	if !found3dTest || !foundFile1 {
		t.Errorf("expected trimmed paths (3dTest/ and 3dTest/file1.txt), got: found3dTest=%v, foundFile1=%v", found3dTest, foundFile1)
	}

	// 2. Тест добавления (append) файла с опцией -trim-parents
	extraFile := filepath.Join(tmp, "downloads", "bass24", "delphi", "extra.txt")
	os.WriteFile(extraFile, []byte("extra"), 0644)

	err = runZipper([]string{"zipper", "a", "-trim-parents", archivePath, extraFile})
	if err != nil {
		t.Fatalf("zipper a with -trim-parents failed: %v", err)
	}

	zr2, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	foundExtra := false
	for _, f := range zr2.File {
		if f.Name == "extra.txt" {
			foundExtra = true
		}
	}
	zr2.Close()

	if !foundExtra {
		t.Error("expected appended extra.txt to be trimmed to base name 'extra.txt'")
	}
}
