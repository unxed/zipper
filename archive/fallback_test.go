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
}type fallbackMockFileInfo struct {
	name string
	mode os.FileMode
}
func (m fallbackMockFileInfo) Name() string { return m.name }
func (m fallbackMockFileInfo) Size() int64 { return 0 }
func (m fallbackMockFileInfo) Mode() os.FileMode { return m.mode }
func (m fallbackMockFileInfo) ModTime() time.Time { return time.Now() }
func (m fallbackMockFileInfo) IsDir() bool { return m.mode.IsDir() }
func (m fallbackMockFileInfo) Sys() interface{} { return nil }

func TestFallbackArchiver_DifferentDrivesWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "different_drives.tar.gz")

	currentDrive := filepath.VolumeName(tmp)
	targetDrive := "D:"
	if currentDrive == "D:" || currentDrive == "d:" {
		targetDrive = "C:"
	}

	targetPath := targetDrive + `\dummy_dir`

	a, err := NewFallbackArchiver(archivePath, tmp, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	files := map[string]os.FileInfo{
		targetPath: fallbackMockFileInfo{name: "dummy_dir", mode: os.ModeDir | 0755},
	}

	err = a.Archive(context.Background(), files)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}
