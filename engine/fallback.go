package engine

import (
    "os"
	"github.com/mholt/archives"
    "fmt"
    "context"
	"path/filepath"
	"strings"
    "io"
    "io/fs"
)

type fallbackExtractor struct {
	filename string
	chroot   string
}

func NewFallbackExtractor(filename, chroot string, opts Options) (Extractor, error) {
	return &fallbackExtractor{filename: filename, chroot: chroot}, nil
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

	return ex.Extract(ctx, stream, func(ctx context.Context, info archives.FileInfo) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Защита от path traversal (Zip Slip)
		targetPath, err := filepath.Abs(filepath.Join(e.chroot, info.NameInArchive))
		if err != nil {
			return err
		}
		prefix := e.chroot
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if !strings.HasPrefix(targetPath, prefix) && targetPath != e.chroot {
			return fmt.Errorf("path traversal attack detected: %s", info.NameInArchive)
		}

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

		_, err = io.Copy(out, in)
		return err
	})
}

func (e *fallbackExtractor) Close() error {
	return nil
}

type fallbackArchiver struct {
	filename string
	chroot   string
	format   archives.Archiver
	f        *os.File
	files    []archives.FileInfo
}

func NewFallbackArchiver(filename, chroot string, opts Options) (Archiver, error) {
	var format archives.Archiver
	lower := strings.ToLower(filename)

	// Назначаем базовые архиваторы, если формат не покрыт нашими основными движками
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		format = archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}}
	} else if strings.HasSuffix(lower, ".gz") {
		format = archives.CompressedArchive{Compression: archives.Gz{}}
	} else if strings.HasSuffix(lower, ".zip") {
		format = archives.Zip{}
	} else {
		return nil, fmt.Errorf("unsupported fallback creation format for %s", filename)
	}

	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	return &fallbackArchiver{
		filename: filename,
		chroot:   chroot,
		format:   format,
		f:        f,
	}, nil
}

func (a *fallbackArchiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	for path, info := range files {
		rel, err := filepath.Rel(a.chroot, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == "" {
			continue
		}

		capturePath := path
		a.files = append(a.files, archives.FileInfo{
			FileInfo:      info,
			NameInArchive: filepath.ToSlash(rel),
			Open: func() (fs.File, error) {
				return os.Open(capturePath)
			},
		})
	}
	return nil
}

func (a *fallbackArchiver) Close() error {
	defer a.f.Close()
	if len(a.files) > 0 {
		return a.format.Archive(context.Background(), a.f, a.files)
	}
	return nil
}