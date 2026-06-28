package archive

import (
	"context"
	"fmt"
	"github.com/unxed/archives"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)
import (
	"sync"
	"sync/atomic"
)

var fallbackCopyBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 256*1024)
		return &b
	},
}

type fallbackExtractor struct {
	filename string
	chroot   string

	writtenBytes   int64
	writtenEntries int64
}

type progressReader struct {
	r io.Reader
	e *fallbackExtractor
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		atomic.AddInt64(&pr.e.writtenBytes, int64(n))
	}
	return n, err
}

func NewFallbackExtractor(filename, chroot string, opts Options) (Extractor, error) {
	absChroot, err := filepath.Abs(chroot)
	if err != nil {
		return nil, err
	}
	return &fallbackExtractor{filename: filename, chroot: absChroot}, nil
}

func (e *fallbackExtractor) Extract(ctx context.Context) error {
	f, err := os.Open(e.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Используем эвристику mholt/archives для определения формата "на лету"
	format, stream, err := archives.Identify(ctx, e.filename, f)
	if err != nil {
		return fmt.Errorf("failed to identify archive format for %s: %w", e.filename, err)
	}

	ex, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("format %T does not support extraction", format)
	}

	cleanChroot := filepath.Clean(e.chroot)

	return ex.Extract(ctx, stream, func(ctx context.Context, info archives.FileInfo) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		atomic.AddInt64(&e.writtenEntries, 1)

		cleanName := filepath.ToSlash(filepath.Clean(info.NameInArchive))
		if strings.HasPrefix(cleanName, "../") || strings.HasPrefix(cleanName, "/") {
			return fmt.Errorf("path traversal attack detected: %s", info.NameInArchive)
		}
		targetPath := filepath.Join(cleanChroot, cleanName)

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer out.Close()

		in, err := info.Open()
		if err != nil {
			return err
		}
		defer in.Close()

		pr := &progressReader{r: in, e: e}

		bufPtr := fallbackCopyBufPool.Get().(*[]byte)
		defer fallbackCopyBufPool.Put(bufPtr)

		_, err = io.CopyBuffer(out, pr, *bufPtr)
		return err
	})
}

func (e *fallbackExtractor) Close() error {
	return nil
}
func (e *fallbackExtractor) Written() (bytes, entries int64) {
	return atomic.LoadInt64(&e.writtenBytes), atomic.LoadInt64(&e.writtenEntries)
}

type fallbackArchiver struct {
	filename string
	chroot   string
	format   archives.Archiver
	f        io.WriteCloser
	opts     Options

	writtenBytes   int64
	writtenEntries int64
}

func NewFallbackArchiver(filename, chroot string, opts Options) (Archiver, error) {
	var format archives.Archiver
	lower := strings.ToLower(filename)

	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		format = archives.CompressedArchive{Compression: archives.Gz{Multithreaded: true}, Archival: archives.Tar{}}
	} else if strings.HasSuffix(lower, ".gz") {
		format = archives.CompressedArchive{Compression: archives.Gz{Multithreaded: true}}
	} else if strings.HasSuffix(lower, ".zip") {
		format = archives.Zip{}
	} else if strings.HasSuffix(lower, ".7z") {
		solid := !opts.NonSolid
		if opts.Solid {
			solid = true // Явный флаг перекрывает умалчивания
		}
		format = archives.SevenZip{Solid: solid, ContinueOnError: opts.Tolerant, Password: opts.Password}
	} else if filename == "-" {
		format = archives.CompressedArchive{Compression: archives.Gz{Multithreaded: true}, Archival: archives.Tar{}}
	} else {
		return nil, fmt.Errorf("unsupported fallback creation format for %s", filename)
	}

	var f io.WriteCloser
	var err error
	if filename == "-" {
		f = stdoutWrapper{os.Stdout}
	} else {
		f, err = os.Create(filename)
		if err != nil {
			return nil, err
		}
	}

	return &fallbackArchiver{
		filename: filename,
		chroot:   chroot,
		format:   format,
		f:        f,
		opts:     opts,
	}, nil
}

type progressFile struct {
	fs.File
	a *fallbackArchiver
}

func (pf *progressFile) Read(p []byte) (int, error) {
	n, err := pf.File.Read(p)
	if n > 0 {
		atomic.AddInt64(&pf.a.writtenBytes, int64(n))
	}
	return n, err
}

func (a *fallbackArchiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	var aFiles []archives.FileInfo
	for path, info := range files {
		var rel string
		var err error
		if a.opts.PathMapping != nil && a.opts.PathMapping[path] != "" {
			rel = a.opts.PathMapping[path]
		} else {
			rel, err = filepath.Rel(a.chroot, path)
			if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
				rel = filepath.ToSlash(path)
				vol := filepath.VolumeName(path)
				if vol != "" {
					rel = strings.TrimPrefix(rel, filepath.ToSlash(vol))
				}
				rel = strings.TrimPrefix(rel, "/")
				err = nil
			}
		}
		if rel == "." || rel == "" {
			continue
		}

		capturePath := path
		aFiles = append(aFiles, archives.FileInfo{
			FileInfo:      info,
			NameInArchive: filepath.ToSlash(rel),
			Open: func() (fs.File, error) {
				f, err := os.Open(capturePath)
				if err == nil {
					atomic.AddInt64(&a.writtenEntries, 1)
					return &progressFile{File: f, a: a}, nil
				}
				return f, err
			},
		})
	}
	if len(aFiles) > 0 {
		return a.format.Archive(ctx, a.f, aFiles)
	}
	return nil
}

func (a *fallbackArchiver) Close() error {
	return a.f.Close()
}

func (a *fallbackArchiver) Written() (bytes, entries int64) {
	return atomic.LoadInt64(&a.writtenBytes), atomic.LoadInt64(&a.writtenEntries)
}
