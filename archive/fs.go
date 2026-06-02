package archive

import (
    "fmt"
    "strings"
	"context"
	"io"
	"io/fs"
	"os"

	"github.com/mholt/archives"
	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

type FileSystem interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS
	io.Closer
}

func OpenFS(filename string, opts Options) (FileSystem, error) {
	fmtType := DetectFormat(filename)
	if fmtType == "zip" {
		return newZipFS(filename, opts)
	} else if fmtType == "tar" {
		return newTarFS(filename, opts)
	}
	return newFallbackFS(filename, opts)
}

func cleanPath(name string) string {
	name = strings.ReplaceAll(name, `\`, `/`)
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "." {
		return "."
	}
	return name
}

type zipFS struct {
	zr *zip.ReadCloser
}

func newZipFS(filename string, opts Options) (FileSystem, error) {
	zr, err := zip.OpenReaderWithPassword(filename, opts.Password)
	if err != nil {
		return nil, err
	}
	return &zipFS{zr: zr}, nil
}

func (z *zipFS) Open(name string) (fs.File, error) {
	return z.zr.Open(cleanPath(name))
}

func (z *zipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(z.zr, cleanPath(name))
}

func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(z.zr, cleanPath(name))
}

func (z *zipFS) Close() error {
	return z.zr.Close()
}

type tarFS struct {
	tfs *tar.TarFS
}

func newTarFS(filename string, opts Options) (FileSystem, error) {
	fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] newTarFS called for filename: %q, opts.IndexPath: %q\n", filename, opts.IndexPath)
	tfs, err := tar.NewFS(filename, opts.IndexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tar.NewFS error: %v\n", err)
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tar.NewFS successfully initialized TarFS pointer: %p\n", tfs)
	return &tarFS{tfs: tfs}, nil
}

func (t *tarFS) Open(name string) (fs.File, error) {
	cleaned := cleanPath(name)
	fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.Open raw: %q, cleaned: %q\n", name, cleaned)
	f, err := t.tfs.Open(cleaned)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.Open error: %v\n", err)
	}
	return f, err
}

func (t *tarFS) ReadDir(name string) ([]fs.DirEntry, error) {
	cleaned := cleanPath(name)
	fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.ReadDir raw: %q, cleaned: %q\n", name, cleaned)
	entries, err := fs.ReadDir(t.tfs, cleaned)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.ReadDir error: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.ReadDir returned %d entries\n", len(entries))
	}
	return entries, err
}

func (t *tarFS) Stat(name string) (fs.FileInfo, error) {
	cleaned := cleanPath(name)
	fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.Stat raw: %q, cleaned: %q\n", name, cleaned)
	info, err := fs.Stat(t.tfs, cleaned)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ZIPPER-ARCHIVE] tarFS.Stat error: %v\n", err)
	}
	return info, err
}

func (t *tarFS) Close() error {
	return t.tfs.Close()
}

type fallbackFS struct {
	f    *os.File
	fsys fs.FS
}

func newFallbackFS(filename string, opts Options) (FileSystem, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	_, _, err = archives.Identify(context.Background(), filename, f)
	if err != nil {
		f.Close()
		return nil, err
	}

	fsys, err := archives.FileSystem(context.Background(), filename, nil)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &fallbackFS{f: f, fsys: fsys}, nil
}

func (f *fallbackFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(cleanPath(name))
}

func (f *fallbackFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, cleanPath(name))
}

func (f *fallbackFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, cleanPath(name))
}

func (f *fallbackFS) Close() error {
	var err1 error
	if closer, ok := f.fsys.(io.Closer); ok {
		err1 = closer.Close()
	}
	err2 := f.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}