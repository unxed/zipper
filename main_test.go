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

	// Тест создания (команда c)
	err := run([]string{"zipper", "c", "-C", srcDir, archivePath, "test.txt"})
	if err != nil {
		t.Fatalf("run(c) failed: %v", err)
	}

	// Ищем фактически созданный файл, т.к. к нему приклеилось расширение
	matches, _ := filepath.Glob(filepath.Join(tmp, "test_cli.*"))
	if len(matches) == 0 {
		t.Fatalf("archive was not created")
	}
	actualArchive := matches[0]

	// Тест извлечения (команда x)
	err = run([]string{"zipper", "x", "-C", dstDir, actualArchive})
	if err != nil {
		t.Fatalf("run(x) failed: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dstDir, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(b) != "cli data" {
		t.Errorf("got %q", string(b))
	}
}