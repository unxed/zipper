package archive

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/unxed/zip"
)

// buildZipBehindStub writes a zip behind an executable stub, the shape of a
// self-extracting archive. With absolute set, the offsets in the central
// directory count the stub in, the way WinRAR writes them; otherwise they are
// counted from the start of the archive, the way a plain concatenation leaves
// them. A reader has to cope with both.
func buildZipBehindStub(t *testing.T, path string, absolute bool, members map[string][]byte) {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range members {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	stub := bytes.Repeat([]byte("MZ this is the stub of the archive "), 64)
	if absolute {
		end := bytes.LastIndex(data, []byte("PK\x05\x06"))
		if end < 0 {
			t.Fatal("end of central directory not found")
		}
		directoryOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
		binary.LittleEndian.PutUint32(data[end+16:end+20], uint32(directoryOffset+len(stub)))
		for p := directoryOffset; p < end; {
			nameLen := int(binary.LittleEndian.Uint16(data[p+28 : p+30]))
			extraLen := int(binary.LittleEndian.Uint16(data[p+30 : p+32]))
			commentLen := int(binary.LittleEndian.Uint16(data[p+32 : p+34]))
			offset := int(binary.LittleEndian.Uint32(data[p+42 : p+46]))
			binary.LittleEndian.PutUint32(data[p+42:p+46], uint32(offset+len(stub)))
			p += 46 + nameLen + extraLen + commentLen
		}
	}

	if err := os.WriteFile(path, append(stub, data...), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A zip behind a stub has neither a zip extension nor a local file header at
// the start of the file, so it used to be left to the generic reader, which
// answers "no formats matched" (f4 issue #1186).
func TestDetectFormatZipBehindStub(t *testing.T) {
	dir := t.TempDir()
	members := map[string][]byte{"member.txt": []byte("behind a stub\n")}

	for _, absolute := range []bool{false, true} {
		name := "relative.exe"
		if absolute {
			name = "absolute.exe"
		}
		path := filepath.Join(dir, name)
		buildZipBehindStub(t, path, absolute, members)
		if got := DetectFormat(path); got != "zip" {
			t.Errorf("DetectFormat(%s) = %q, want zip", name, got)
		}

		fsys, err := OpenFS(path, Options{})
		if err != nil {
			t.Fatalf("OpenFS(%s): %v", name, err)
		}
		f, err := fsys.Open("member.txt")
		if err != nil {
			t.Fatalf("%s: open member: %v", name, err)
		}
		got, err := io.ReadAll(f)
		_ = f.Close()
		_ = fsys.Close()
		if err != nil || !bytes.Equal(got, members["member.txt"]) {
			t.Errorf("%s: member read back as %q (err %v)", name, got, err)
		}
	}

	plain := filepath.Join(dir, "plain.bin")
	if err := os.WriteFile(plain, bytes.Repeat([]byte("not an archive at all\n"), 100), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := DetectFormat(plain); got != "" {
		t.Errorf("DetectFormat(a file that is not an archive) = %q, want empty", got)
	}
}
