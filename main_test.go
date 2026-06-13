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
func TestTarMimicry_Password(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "test.txt")
	os.WriteFile(srcFile, []byte("protected tar data"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	arc := "protected.tar.gz"
	err := runTar([]string{"tar", "-c", "-z", "-P", "pass", "-f", arc, "test.txt"})
	if err != nil {
		t.Fatalf("tar create failed: %v", err)
	}

	os.Remove("test.txt")

	err = runTar([]string{"tar", "-x", "-z", "-P", "pass", "-f", arc})
	if err != nil {
		t.Fatalf("tar extract failed: %v", err)
	}

	b, _ := os.ReadFile("test.txt")
	if string(b) != "protected tar data" {
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
func TestCliMultiVolume(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	os.MkdirAll(srcDir, 0755)

	// Создаем тестовые данные размером 50 КБ
	data := make([]byte, 50*1024)
	for i := range data {
		data[i] = byte('A' + (i % 26))
	}
	os.WriteFile(filepath.Join(srcDir, "large.txt"), data, 0644)

	archivePath := filepath.Join(tmp, "split_archive.zip")

	// Эмулируем запуск создания многотомного ZIP-архива без сжатия (-m store) с шагом тома в 10 КБ
	err := runZipper([]string{"zipper", "c", "-C", srcDir, "-v", "10K", "-m", "store", archivePath, "large.txt"})
	if err != nil {
		t.Fatalf("failed to create split archive via CLI: %v", err)
	}

	// Проверяем физическое наличие томов
	prefix := archivePath[:len(archivePath)-len(".zip")]
	if _, err := os.Stat(prefix + ".z01"); err != nil {
		t.Error("missing volume .z01 on disk")
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Error("missing main volume .zip on disk")
	}

	// Эмулируем запуск извлечения архива
	os.MkdirAll(dstDir, 0755)
	err = runZipper([]string{"zipper", "x", "-C", dstDir, archivePath})
	if err != nil {
		t.Fatalf("failed to extract split archive via CLI: %v", err)
	}

	extractedData, err := os.ReadFile(filepath.Join(dstDir, "large.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(extractedData) != string(data) {
		t.Error("extracted split archive content mismatch with original data")
	}
}

func TestZipMimicry_Password(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "test.txt")
	os.WriteFile(srcFile, []byte("protected data"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	arc := "protected"
	err := runZip([]string{"zip", "-P", "pass", arc, "test.txt"})
	if err != nil {
		t.Fatalf("zip create failed: %v", err)
	}

	os.Remove("test.txt")

	err = runUnzip([]string{"unzip", "-P", "pass", arc + ".zip", "-d", "out"})
	if err != nil {
		t.Fatalf("unzip extract failed: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join("out", "test.txt"))
	if string(b) != "protected data" {
		t.Errorf("content mismatch: got %q", string(b))
	}
}
func TestCliAppendAndDelete(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "archive.zip")

	// 1. Create initial zip with "file1.txt"
	os.WriteFile(filepath.Join(tmp, "file1.txt"), []byte("data1"), 0644)
	err := runZipper([]string{"zipper", "c", "-C", tmp, arc, "file1.txt"})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Append "file2.txt" using "zipper a"
	os.WriteFile(filepath.Join(tmp, "file2.txt"), []byte("data2"), 0644)
	err = runZipper([]string{"zipper", "a", "-C", tmp, arc, "file2.txt"})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Delete "file1.txt" using "zipper d"
	err = runZipper([]string{"zipper", "d", "-C", tmp, arc, "file1.txt"})
	if err != nil {
		t.Fatal(err)
	}

	// 4. Extract and verify that only "file2.txt" exists
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(dst, 0755)
	err = runZipper([]string{"zipper", "x", "-C", dst, arc})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, "file1.txt")); !os.IsNotExist(err) {
		t.Error("expected file1.txt to be deleted from the archive")
	}
	b2, err := os.ReadFile(filepath.Join(dst, "file2.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) != "data2" {
		t.Errorf("got %q, want 'data2'", string(b2))
	}
}
func TestCliExternalRecoveryRecord(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "archive_ext.zip")

	os.WriteFile(filepath.Join(tmp, "file1.txt"), []byte("data for external par2 recovery testing"), 0644)
	// Create with external recovery record (-rr 10 -rr-external)
	err := runZipper([]string{"zipper", "c", "-C", tmp, "-rr", "10", "-rr-external", arc, "file1.txt"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the external .par2 file was created next to the archive
	parPath := arc + ".par2"
	if _, err := os.Stat(parPath); err != nil {
		t.Errorf("expected external par2 file at %s, but got: %v", parPath, err)
	}

	// Corrupt the archive slightly
	raw, _ := os.ReadFile(arc)
	for i := 40; i < 45 && i < len(raw); i++ {
		raw[i] = 0x00
	}
	os.WriteFile(arc, raw, 0644)

	// Repair using "zipper repair" (should pick up external .par2 file automatically)
	err = runZipper([]string{"zipper", "repair", arc})
	if err != nil {
		t.Fatalf("repair using external par2 failed: %v", err)
	}

	// Extract and verify integrity
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(dst, 0755)
	err = runZipper([]string{"zipper", "x", "-C", dst, arc})
	if err != nil {
		t.Fatalf("failed to extract repaired archive: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dst, "file1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "data for external par2 recovery testing" {
		t.Errorf("got %q, want 'data for external par2...'", string(b))
	}
}
