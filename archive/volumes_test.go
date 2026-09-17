package archive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeVolumes(t *testing.T, stem string, parts ...[]byte) string {
	t.Helper()
	for i, part := range parts {
		name := stem + "." + []string{"001", "002", "003", "004"}[i]
		if err := os.WriteFile(name, part, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return stem + ".001"
}

func TestOpenInputReadsVolumesAsOneStream(t *testing.T) {
	first := writeVolumes(t, filepath.Join(t.TempDir(), "data.bin"), []byte("ab"), nil, []byte("cdef"))

	in, err := OpenInput(first)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if !in.Split() || len(in.Volumes()) != 3 || in.Size() != 6 {
		t.Fatalf("split=%t volumes=%v size=%d; want 3 volumes, 6 bytes", in.Split(), in.Volumes(), in.Size())
	}

	buf := make([]byte, 4)
	if n, err := in.ReadAt(buf, 1); err != nil || string(buf[:n]) != "bcde" {
		t.Fatalf("ReadAt(4 @1) = %q, %v; want \"bcde\" across an empty volume", buf[:n], err)
	}
	if n, err := in.ReadAt(buf, 4); !errors.Is(err, io.EOF) || string(buf[:n]) != "ef" {
		t.Fatalf("ReadAt(4 @4) = %q, %v; want \"ef\" and io.EOF", buf[:n], err)
	}
	if _, err := in.ReadAt(buf, 6); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt past the end: err = %v, want io.EOF", err)
	}
	all, err := io.ReadAll(in)
	if err != nil || string(all) != "abcdef" {
		t.Fatalf("ReadAll = %q, %v", all, err)
	}
}

func TestOpenInputSingleFileIsNotSplit(t *testing.T) {
	name := filepath.Join(t.TempDir(), "plain.7z")
	if err := os.WriteFile(name, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := OpenInput(name)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if in.Split() || in.Size() != 5 {
		t.Fatalf("split=%t size=%d; want a single 5-byte input", in.Split(), in.Size())
	}
}

// A 7z archive split into volumes (7z -v) must list and extract from its first
// volume; with only that volume read the header lies past the end
// ("sevenzip: error reading header id: EOF", f4 issue #1179).
func TestSplit7zVolumesListAndExtract(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(strings.Repeat("split 7z volume payload\n", 4096))
	if err := os.WriteFile(filepath.Join(src, "member.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	whole := filepath.Join(tmp, "whole.7z")
	a, err := NewFallbackArchiver(whole, src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(src, "member.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), map[string]os.FileInfo{filepath.Join(src, "member.txt"): info}); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(whole)
	if err != nil {
		t.Fatal(err)
	}
	third := len(data) / 3
	first := writeVolumes(t, filepath.Join(tmp, "split.7z"), data[:third], data[third:2*third], data[2*third:])

	fsys, err := OpenFS(first, Options{})
	if err != nil {
		t.Fatalf("OpenFS(%s): %v", filepath.Base(first), err)
	}
	entries, err := fsys.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "member.txt" {
		t.Fatalf("entries = %v, want member.txt", entries)
	}
	member, err := fsys.Open("member.txt")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := io.ReadAll(member)
	_ = member.Close()
	if err != nil || !bytes.Equal(listed, content) {
		t.Fatalf("read member through OpenFS: %d bytes, err %v", len(listed), err)
	}
	if err := fsys.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	e, err := NewExtractor(first, dst, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Extract(context.Background()); err != nil {
		t.Fatalf("Extract(%s): %v", filepath.Base(first), err)
	}
	_ = e.Close()
	extracted, err := os.ReadFile(filepath.Join(dst, "member.txt"))
	if err != nil || !bytes.Equal(extracted, content) {
		t.Fatalf("extracted member.txt: %d bytes, err %v", len(extracted), err)
	}
}
