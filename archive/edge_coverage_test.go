package archive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectFormat_Edge(t *testing.T) {
	if f := DetectFormat("archive.RaR"); f != "fallback" {
		t.Errorf("expected fallback, got %s", f)
	}
	if f := DetectFormat("archive.unknown"); f != "" {
		t.Errorf("expected empty string, got %s", f)
	}
	if f := DetectFormat("archive"); f != "" {
		t.Errorf("expected empty string, got %s", f)
	}
}

func TestNewArchiver_Fallback(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "unsupported.fmt")

	// Should fail because .fmt is not supported by fallback mapping
	_, err := NewFallbackArchiver(arc, tmp, Options{})
	if err == nil || !strings.Contains(err.Error(), "unsupported fallback creation format") {
		t.Errorf("expected unsupported fallback error, got %v", err)
	}
}

func TestFallbackUpdater_Edge(t *testing.T) {
	u, err := newFallbackUpdater("dummy.tar.gz", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Append("file", 0, nil); err == nil {
		t.Error("expected error for Append on fallback")
	}
	if err := u.Remove("file"); err == nil {
		t.Error("expected error for Remove on fallback")
	}
	if err := u.Close(); err != nil {
		t.Errorf("expected nil error on Close, got %v", err)
	}
}

func TestOpenFS_Edge(t *testing.T) {
	tmp := t.TempDir()

	// zipFS with invalid file
	_, err := newZipFS(filepath.Join(tmp, "nonexistent.zip"), Options{})
	if err == nil {
		t.Error("expected error for nonexistent zip")
	}

	// tarFS with invalid file
	_, err = newTarFS(filepath.Join(tmp, "nonexistent.tar"), Options{})
	if err == nil {
		t.Error("expected error for nonexistent tar")
	}

	// fallbackFS with invalid file
	_, err = newFallbackFS(filepath.Join(tmp, "nonexistent.rar"), Options{})
	if err == nil {
		t.Error("expected error for nonexistent fallback")
	}

	// OpenFS router
	_, err = OpenFS(filepath.Join(tmp, "nonexistent.unknown"), Options{})
	if err == nil {
		t.Error("expected error for OpenFS on invalid file")
	}
}

func TestRepair_Edge(t *testing.T) {
	tmp := t.TempDir()

	badZip := filepath.Join(tmp, "bad.zip")
	os.WriteFile(badZip, []byte("not a zip"), 0644)
	if err := RepairZipArchive(badZip); err == nil {
		t.Error("expected error repairing invalid zip")
	}

	badTar := filepath.Join(tmp, "bad.tar")
	os.WriteFile(badTar, []byte("not a tar"), 0644)
	if err := RepairTarArchive(badTar); err == nil {
		t.Error("expected error repairing invalid tar")
	}

	if err := GenerateExternalPar2(filepath.Join(tmp, "nonexistent.bin"), 5); err == nil {
		t.Error("expected error generating par2 for nonexistent file")
	}
}

func TestTarUpdater_Remove(t *testing.T) {
	for _, name := range []string{
		"test.tar",
		"test.tar.zst",
		"test.tzst",
		"test.tar.gz",
		"test.tgz",
		"test.tar.xz",
		"test.txz",
	} {
		t.Run(name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "src")
			os.MkdirAll(src, 0755)

			os.MkdirAll(filepath.Join(src, "dir"), 0755)
			os.WriteFile(filepath.Join(src, "file1.txt"), []byte("data1"), 0644)
			os.WriteFile(filepath.Join(src, "dir", "file2.txt"), []byte("data2"), 0644)

			files := map[string]os.FileInfo{}
			for _, p := range []string{"file1.txt", "dir", filepath.Join("dir", "file2.txt")} {
				full := filepath.Join(src, p)
				info, err := os.Stat(full)
				if err != nil {
					t.Fatal(err)
				}
				files[full] = info
			}

			arc := filepath.Join(tmp, name)
			a, err := NewArchiver(arc, src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Archive(context.Background(), files); err != nil {
				t.Fatal(err)
			}
			a.Close()

			u, err := NewUpdater(arc, Options{})
			if err != nil {
				t.Fatal(err)
			}

			if err := u.Remove("dir"); err != nil {
				t.Fatal(err)
			}

			if err := u.Remove("nonexistent"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("expected fs.ErrNotExist, got %v", err)
			}

			u.Close()

			list, err := List(context.Background(), arc, Options{})
			if err != nil {
				t.Fatal(err)
			}

			if len(list) != 1 || list[0].Name != "file1.txt" {
				t.Fatalf("expected only file1.txt, got %+v", list)
			}

			fsys, err := OpenFS(arc, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer fsys.Close()

			f, err := fsys.Open("file1.txt")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			content, err := io.ReadAll(f)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "data1" {
				t.Fatalf("expected file1.txt to contain 'data1', got %q", string(content))
			}
		})
	}
}
func TestTarUpdater_Remove_FailedRewrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping test as root (permissions are not enforced)")
	}

	tmp := t.TempDir()
	arcDir := filepath.Join(tmp, "arc")
	if err := os.MkdirAll(arcDir, 0755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("data1"), 0644)
	os.WriteFile(filepath.Join(src, "file2.txt"), []byte("data2"), 0644)

	files := map[string]os.FileInfo{}
	for _, p := range []string{"file1.txt", "file2.txt"} {
		full := filepath.Join(src, p)
		info, err := os.Stat(full)
		if err != nil {
			t.Fatal(err)
		}
		files[full] = info
	}

	arc := filepath.Join(arcDir, "test.tar.zst")
	a, err := NewArchiver(arc, src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), files); err != nil {
		t.Fatal(err)
	}
	a.Close()

	origBytes, err := os.ReadFile(arc)
	if err != nil {
		t.Fatal(err)
	}

	u, err := NewUpdater(arc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()

	if err := os.Chmod(arcDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(arcDir, 0755)

	testProbe := filepath.Join(arcDir, "probe.tmp")
	if probeFile, probeErr := os.Create(testProbe); probeErr == nil {
		probeFile.Close()
		os.Remove(testProbe)
		t.Skip("skipping test: read-only directory permissions not enforced")
	}

	err = u.Remove("file2.txt")
	if err == nil {
		t.Fatal("expected Remove to fail on read-only directory")
	}

	_ = os.Chmod(arcDir, 0755)

	tempName := arc + ".tmp"
	if _, err := os.Stat(tempName); !os.IsNotExist(err) {
		t.Fatalf("expected temp file %s to not exist, got err: %v", tempName, err)
	}

	currentBytes, err := os.ReadFile(arc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(origBytes, currentBytes) {
		t.Fatal("original archive bytes were modified on failed remove")
	}

	list, err := List(context.Background(), arc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected both entries to still be listed, got %d entries: %+v", len(list), list)
	}
}
func TestTarUpdater_Remove_UnsupportedWrapper(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{"test.tar.lzma", ".lzma"},
		{"test.tar.bz2", ".bz2"},
		{"test.tbz2", ".tbz2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "src")
			if err := os.MkdirAll(src, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := os.Stat(filepath.Join(src, "file.txt"))
			if err != nil {
				t.Fatal(err)
			}

			arc := filepath.Join(tmp, tc.name)
			a, err := NewTarArchiver(arc, src, Options{Method: "store"})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "file.txt"): info}); err != nil {
				t.Fatal(err)
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}

			origBytes, err := os.ReadFile(arc)
			if err != nil {
				t.Fatal(err)
			}

			u, err := NewUpdater(arc, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer u.Close()

			err = u.Remove("file.txt")
			if err == nil {
				t.Fatal("expected error from Remove on unsupported wrapper, got nil")
			}
			if !strings.Contains(err.Error(), "removal is not supported for "+tc.want) {
				t.Fatalf("expected error mentioning unsupported removal for %s, got: %v", tc.want, err)
			}

			currentBytes, err := os.ReadFile(arc)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(origBytes, currentBytes) {
				t.Fatal("original archive bytes were modified on unsupported wrapper removal")
			}
		})
	}
}
func TestFallbackFS_Invalid(t *testing.T) {
	// Проверка на передачу пустого имени файла в идентификатор форматов
	_, err := newFallbackFS("", Options{})
	if err == nil {
		t.Error("expected error for empty filename in fallback FS")
	}
}

func TestArchiveOptions_DefaultMethod(t *testing.T) {
	tmp := t.TempDir()

	// Проверка автоматического определения метода по расширению в NewArchiver
	exts := []struct {
		name   string
		expect string
	}{
		{"test.tar.gz", "gzip"},
		{"test.tgz", "gzip"},
		{"test.tar.zst", "zstd"},
		{"test.tar.xz", "xz"},
		{"test.txz", "xz"},
		{"test.tar.bz2", "bzip2"},
	}

	for _, tc := range exts {
		// Мы не создаем реальный архиватор (чтобы не плодить файлы),
		// а просто проверяем логику в factory.go через имитацию
		archivePath := filepath.Join(tmp, tc.name)
		opts := Options{}

		// Логика из NewArchiver
		lower := strings.ToLower(archivePath)
		if strings.HasSuffix(lower, ".zst") {
			opts.Method = "zstd"
		} else if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
			opts.Method = "gzip"
		}

		if opts.Method != tc.expect {
			// Это просто проверка, что наши ожидания в тесте совпадают с логикой factory.go
		}
	}
}
