package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/unxed/archives"
	"github.com/unxed/zip"
)

// TestError is a failure while reading one archive member.
type TestError struct {
	Name string
	Err  error
}

type testProgress struct {
	bytes   int64
	entries int64
	fn      TestProgressFunc
}

func (p *testProgress) addBytes(n int64) {
	if n > 0 {
		atomic.AddInt64(&p.bytes, n)
		p.report()
	}
}

func (p *testProgress) addEntry() {
	atomic.AddInt64(&p.entries, 1)
	p.report()
}

func (p *testProgress) report() {
	if p.fn != nil {
		p.fn(atomic.LoadInt64(&p.bytes), atomic.LoadInt64(&p.entries))
	}
}

type testReader struct {
	ctx context.Context
	r   io.Reader
	p   *testProgress
}

func (r testReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(b)
	r.p.addBytes(int64(n))
	return n, err
}

// Test reads every file in an archive to EOF without extracting it. ZIP data
// is read through the ZIP reader so its CRC is checked; other formats use the
// archives extractor and stream each member to io.Discard. Per-file failures
// are returned in TestError; errors opening or identifying the archive are
// returned as the function error. Progress, when non-nil, reports cumulative
// uncompressed bytes read and files tested.
func Test(ctx context.Context, filename string, opts Options, progress TestProgressFunc) ([]TestError, error) {
	p := &testProgress{fn: progress}
	if DetectFormat(filename) == "zip" && filename != "-" {
		return testZip(ctx, filename, opts, p)
	}
	return testFallback(ctx, filename, opts, p)
}

func testZip(ctx context.Context, filename string, opts Options, progress *testProgress) ([]TestError, error) {
	zr, err := zip.OpenReaderWithPassword(filename, opts.Password)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var failures []TestError
	for _, file := range zr.File {
		if err := ctx.Err(); err != nil {
			return failures, err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		progress.addEntry()
		r, err := file.Open()
		if err == nil {
			_, err = io.Copy(io.Discard, testReader{ctx: ctx, r: r, p: progress})
			closeErr := r.Close()
			if err == nil {
				err = closeErr
			}
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return failures, ctxErr
		}
		if err != nil {
			failures = append(failures, TestError{Name: file.Name, Err: err})
		}
	}
	return failures, nil
}

func testFallback(ctx context.Context, filename string, opts Options, progress *testProgress) ([]TestError, error) {
	in, err := OpenInput(filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	format, stream, err := archives.Identify(ctx, filename, in)
	if err != nil {
		return nil, fmt.Errorf("failed to identify archive format for %s: %w", filename, err)
	}
	var ex archives.Extractor
	if passwordFormat, ok := fallbackPasswordFormat(format, opts.Password); ok {
		ex = passwordFormat
	} else {
		ex, _ = format.(archives.Extractor)
	}
	if ex == nil {
		return nil, fmt.Errorf("format %T does not support testing", format)
	}

	var failures []TestError
	err = ex.Extract(ctx, stream, func(ctx context.Context, info archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		progress.addEntry()
		r, err := info.Open()
		if err == nil {
			_, err = io.Copy(io.Discard, testReader{ctx: ctx, r: r, p: progress})
			closeErr := r.Close()
			if err == nil {
				err = closeErr
			}
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			failures = append(failures, TestError{Name: info.NameInArchive, Err: err})
		}
		return nil
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return failures, ctxErr
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		return failures, err
	}
	return failures, err
}
