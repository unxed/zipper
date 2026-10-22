package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractorCancellationBetweenFiles(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	files := make(map[string]os.FileInfo, 100)
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("%03d.txt", i)
		data := []byte("x")
		if i == 0 {
			// Keep the first extraction in progress long enough for the test to
			// observe its completion and cancel before the next file is written.
			data = make([]byte, 8<<20)
		}
		filename := filepath.Join(src, name)
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatal(err)
		}
		files[filename] = info
	}

	archivePath := filepath.Join(tmp, "hundred.zip")
	a, err := NewArchiver(archivePath, src, Options{Method: "store", Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(context.Background(), files); err != nil {
		_ = a.Close()
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	e, err := NewExtractor(archivePath, filepath.Join(tmp, "dst"), Options{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- e.Extract(ctx) }()

	deadline := time.NewTimer(10 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		_, entries := e.Written()
		if entries > 0 {
			cancel()
			break
		}
		select {
		case err := <-result:
			t.Fatalf("extraction finished before a file was written: %v", err)
		case <-deadline.C:
			cancel()
			t.Fatal("timed out waiting for the first extracted file")
		case <-ticker.C:
		}
	}

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Extract() error = %v, want context.Canceled", err)
	}
	_, entries := e.Written()
	if entries >= 100 {
		t.Fatalf("extracted %d entries after cancellation, want fewer than 100", entries)
	}
}
