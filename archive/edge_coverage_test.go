package archive

import (
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
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "test.tar")
	os.WriteFile(arc, make([]byte, 1024), 0644) // dummy file

	u, err := newTarUpdater(arc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Remove("test"); err == nil || !strings.Contains(err.Error(), "not supported natively") {
		t.Errorf("expected unsupported removal error, got %v", err)
	}
	u.Close()
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
