package archive

import (
	"os"
	"path/filepath"
	"testing"
)

// The parts of a ZIP split archive are .z01, .z02 and so on, and the first of
// them starts with the spanning marker instead of a local file header, so
// neither the extension list nor the magic used to place them as zip
// (f4 issue #1186).
func TestDetectFormatSplitZipVolumes(t *testing.T) {
	dir := t.TempDir()
	for name, want := range map[string]string{
		"archive.z01":  "zip",
		"archive.z09":  "zip",
		"archive.Z10":  "zip",
		"archive.z100": "zip",
		"archive.zip":  "zip",
		"archive.z":    "",
		"archive.zx1":  "",
		"archive.7z":   "fallback",
	} {
		if got := DetectFormat(filepath.Join(dir, name)); got != want {
			t.Errorf("DetectFormat(%s) = %q, want %q", name, got, want)
		}
	}

	spanning := filepath.Join(dir, "volume.part1")
	if err := os.WriteFile(spanning, []byte("PK\x07\x08rest of the volume"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := DetectFormat(spanning); got != "zip" {
		t.Errorf("DetectFormat(a file starting with the spanning marker) = %q, want zip", got)
	}
}
