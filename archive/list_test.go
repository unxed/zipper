package archive

import (
	"context"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listSample compresses well, so a compressed entry is smaller than it.
var listSample = strings.Repeat("hello ", 1000)

// listArchive packs a.txt, sub/ and sub/b.txt into a new archive with the
// given name and options, and lists it with the same options.
func listArchive(t *testing.T, name string, opts Options) map[string]Entry {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte(listSample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]os.FileInfo{}
	for _, rel := range []string{"a.txt", "sub", filepath.Join("sub", "b.txt")} {
		p := filepath.Join(src, rel)
		info, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		files[p] = info
	}
	arc := filepath.Join(t.TempDir(), name)
	a, err := NewArchiver(arc, src, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), files); err != nil {
		a.Close()
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	list, err := List(context.Background(), arc, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	var names []string
	for _, e := range list {
		got[e.Name] = e
		names = append(names, e.Name)
	}
	if strings.Join(names, ",") != "a.txt,sub,sub/b.txt" {
		t.Fatalf("List(%s) names = %v, want [a.txt sub sub/b.txt]", name, names)
	}
	if !got["sub"].IsDir || got["a.txt"].IsDir || got["sub/b.txt"].IsDir {
		t.Fatalf("List(%s) IsDir wrong: %+v", name, list)
	}
	if got["a.txt"].Size != int64(len(listSample)) || got["sub/b.txt"].Size != 5 || got["sub"].Size != 0 {
		t.Fatalf("List(%s) sizes wrong: %+v", name, list)
	}
	if got["a.txt"].Modified.IsZero() {
		t.Fatalf("List(%s): a.txt has no modification time", name)
	}
	return got
}

func TestListZipHeaders(t *testing.T) {
	got := listArchive(t, "x.zip", Options{Method: "deflate"})
	a := got["a.txt"]
	if a.Method != "deflate" || a.Encrypted {
		t.Errorf("a.txt = %+v, want deflate, not encrypted", a)
	}
	if a.Packed <= 0 || a.Packed >= a.Size {
		t.Errorf("a.txt Packed = %d, want between 0 and %d (the text compresses)", a.Packed, a.Size)
	}
	if want := crc32.ChecksumIEEE([]byte(listSample)); a.CRC32 != want {
		t.Errorf("a.txt CRC32 = %08x, want %08x", a.CRC32, want)
	}
	if b := got["sub/b.txt"]; b.CRC32 != crc32.ChecksumIEEE([]byte("world")) {
		t.Errorf("sub/b.txt CRC32 = %08x", b.CRC32)
	}
	if d := got["sub"]; d.Method != "" || d.Packed < 0 {
		t.Errorf("sub = %+v, want no method and a known Packed", d)
	}
}

func TestListZipEncrypted(t *testing.T) {
	got := listArchive(t, "x.zip", Options{Method: "deflate", Password: "12345"})
	a := got["a.txt"]
	if !a.Encrypted {
		t.Errorf("a.txt = %+v, want Encrypted", a)
	}
	if a.Method == "aes" || strings.HasPrefix(a.Method, "method ") {
		t.Errorf("a.txt Method = %q, want the method under the encryption", a.Method)
	}
}

func TestListTar(t *testing.T) {
	for name, method := range map[string]string{"x.tar": "store", "x.tar.zst": "zstd", "x.tar.gz": "gzip", "x.tar.xz": "xz"} {
		t.Run(name, func(t *testing.T) {
			got := listArchive(t, name, Options{})
			for _, e := range got {
				if e.Packed != -1 || e.CRC32 != 0 || e.Encrypted {
					t.Errorf("%s = %+v, want Packed -1, CRC32 0, not encrypted", e.Name, e)
				}
				if e.Method != method {
					t.Errorf("%s Method = %q, want %q", e.Name, e.Method, method)
				}
			}
		})
	}
}

func TestList7z(t *testing.T) {
	got := listArchive(t, "x.7z", Options{})
	for _, e := range got {
		if e.Packed != -1 || e.CRC32 != 0 || e.Method != "" {
			t.Errorf("%s = %+v, want Packed -1, CRC32 0, no method", e.Name, e)
		}
	}
}

func TestListStopsOnCancel(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := t.TempDir()
	p := filepath.Join(src, "a.txt")
	if err := os.WriteFile(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	arc := filepath.Join(t.TempDir(), "x.zip")
	a, err := NewArchiver(arc, src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), map[string]os.FileInfo{p: info}); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := List(ctx, arc, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("List with a cancelled context = %v, want context.Canceled", err)
	}
}

func TestListMissingArchive(t *testing.T) {
	if _, err := List(context.Background(), filepath.Join(t.TempDir(), "absent.zip"), Options{}); err == nil {
		t.Fatal("List of a missing archive returned no error")
	}
}
