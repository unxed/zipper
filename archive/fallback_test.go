package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFallbackEngine(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("fallback data"), 0644)

	// Создаем архив, который обрабатывается fallback-движком (например, используя .tar.gz вручную)
	// Хотя DetectFormat и не отправит .tar.gz в fallback, мы можем использовать
	// fallback-конструктор напрямую для тестирования реализации.
	arc := filepath.Join(tmp, "test_fallback.tar.gz")

	a, err := NewFallbackArchiver(arc, src, Options{})
	if err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(filepath.Join(src, "test.txt"))
	a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "test.txt"): info})
	a.Close()

	os.MkdirAll(dst, 0755)
	// Для распаковки используем NewFallbackExtractor напрямую
	e, err := NewFallbackExtractor(arc, dst, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Extract(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.Close()

	b, _ := os.ReadFile(filepath.Join(dst, "test.txt"))
	if string(b) != "fallback data" {
		t.Errorf("expected 'fallback data', got %q", string(b))
	}
}