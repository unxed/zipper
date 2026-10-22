package archive

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func makeTestZip(t *testing.T) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "sample.zip")
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	entry, err := zw.CreateHeader(&zip.FileHeader{Name: "file.txt", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("archive contents")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestTestValidZip(t *testing.T) {
	filename := makeTestZip(t)
	failures, err := Test(context.Background(), filename, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Fatalf("Test() errors = %+v, want none", failures)
	}
}

func TestTestCorruptZipReportsMember(t *testing.T) {
	filename := makeTestZip(t)
	r, err := zip.OpenReader(filename)
	if err != nil {
		t.Fatal(err)
	}
	dataOffset, err := r.File[0].DataOffset()
	if err != nil {
		r.Close()
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if _, err := f.ReadAt(b[:], dataOffset); err != nil {
		f.Close()
		t.Fatal(err)
	}
	b[0] ^= 0xff
	if _, err := f.WriteAt(b[:], dataOffset); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	failures, err := Test(context.Background(), filename, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || failures[0].Name != "file.txt" || failures[0].Err == nil {
		t.Fatalf("Test() errors = %+v, want one error for file.txt", failures)
	}
}
