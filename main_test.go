package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCliCreateAndExtract(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(srcDir, 0755)

	testFile := filepath.Join(srcDir, "test.txt")
	os.WriteFile(testFile, []byte("cli data"), 0644)

	// Намеренно передаем без расширения, чтобы проверить подстановку дефолтного
	archivePath := filepath.Join(tmp, "test_cli")

	// Тест создания (команда c) через базовый интерфейс (эмуляция бинарника zipper)
	err := runZipper([]string{"zipper", "c", "-C", srcDir, archivePath, "test.txt"})
	if err != nil {
		t.Fatalf("runZipper(c) failed: %v", err)
	}

	// Ищем фактически созданный файл, т.к. к нему приклеилось расширение
	matches, _ := filepath.Glob(filepath.Join(tmp, "test_cli.*"))
	if len(matches) == 0 {
		t.Fatalf("archive was not created")
	}
	actualArchive := matches[0]

	// Тест извлечения (команда x) через базовый интерфейс
	err = runZipper([]string{"zipper", "x", "-C", dstDir, actualArchive})
	if err != nil {
		t.Fatalf("runZipper(x) failed: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dstDir, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(b) != "cli data" {
		t.Errorf("got %q", string(b))
	}
}

func TestTarMimicry(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "test.txt")
	os.WriteFile(srcFile, []byte("tar mimicry data"), 0644)

	// Меняем рабочую директорию, т.к. эмуляторы работают относительно нее
	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	arc := "test.tar.gz"
	err := runTar([]string{"tar", "-czf", arc, "test.txt"})
	if err != nil {
		t.Fatalf("tar create failed: %v", err)
	}

	os.Remove("test.txt")

	err = runTar([]string{"tar", "-xzf", arc})
	if err != nil {
		t.Fatalf("tar extract failed: %v", err)
	}

	b, _ := os.ReadFile("test.txt")
	if string(b) != "tar mimicry data" {
		t.Errorf("content mismatch: got %q", string(b))
	}
}

func TestZipMimicry(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "test.txt")
	os.WriteFile(srcFile, []byte("zip mimicry data"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	arc := "test_zip" // Без расширения
	err := runZip([]string{"zip", "-r", "-0", arc, "test.txt"})
	if err != nil {
		t.Fatalf("zip create failed: %v", err)
	}

	os.Remove("test.txt")

	err = runUnzip([]string{"unzip", arc + ".zip", "-d", "out"})
	if err != nil {
		t.Fatalf("unzip extract failed: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join("out", "test.txt"))
	if string(b) != "zip mimicry data" {
		t.Errorf("content mismatch: got %q", string(b))
	}
}
