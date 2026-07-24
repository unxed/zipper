package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/unxed/archives"
	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

type FileSystem interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS
	io.Closer
}

type spoolFS struct {
	FileSystem
	tempFile string
}

func (s *spoolFS) Close() error {
	err := s.FileSystem.Close()
	if s.tempFile != "" {
		os.Remove(s.tempFile)
	}
	return err
}

func OpenFS(filename string, opts Options) (FileSystem, error) {
	var tempFile string
	originalFilename := filename

	if filename == "-" {
		var err error
		tempFile, err = SpoolStdin()
		if err != nil {
			return nil, err
		}
		filename = tempFile
	}

	fmtType := DetectFormat(originalFilename)
	if fmtType == "" && tempFile != "" {
		fmtType = DetectFormat(tempFile)
	}

	var fsys FileSystem
	var err error
	if fmtType == "zip" {
		fsys, err = newZipFS(filename, opts)
	} else if fmtType == "tar" {
		fsys, err = newTarFS(filename, opts)
	} else {
		fsys, err = newFallbackFS(filename, opts)
	}

	if err != nil {
		if tempFile != "" {
			os.Remove(tempFile)
		}
		return nil, err
	}

	if tempFile != "" {
		return &spoolFS{FileSystem: fsys, tempFile: tempFile}, nil
	}
	return fsys, nil
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
	return z.zr.Open(name)
}

func (z *zipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(z.zr, name)
}

func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(z.zr, name)
}

func (z *zipFS) Close() error {
	return z.zr.Close()
}

type tarFS struct {
	tfs *tar.TarFS
}

func newTarFS(filename string, opts Options) (FileSystem, error) {
	var fopts []tar.FSOption
	if opts.Password != "" {
		fopts = append(fopts, tar.WithFSPassword(opts.Password))
	}
	indexPath := opts.IndexPath
	if indexPath == "" {
		for _, ext := range []string{".index.sqlite", ".index.arcidx", ".arcidx"} {
			candidate := filename + ext
			if _, err := os.Stat(candidate); err == nil {
				indexPath = candidate
				break
			}
		}
	}
	if indexPath == "" {
		indexPath = getDeterministicIndexPath(filename)
	}
	tfs, err := tar.NewFS(filename, indexPath, fopts...)
	if err != nil {
		return nil, err
	}
	return &tarFS{tfs: tfs}, nil
}

func (t *tarFS) Open(name string) (fs.File, error) {
	return t.tfs.Open(name)
}

func (t *tarFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(t.tfs, name)
}

func (t *tarFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(t.tfs, name)
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
	return f.fsys.Open(name)
}

func (f *fallbackFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, name)
}

func (f *fallbackFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, name)
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

func getDeterministicIndexPath(filename string) string {
	abs, err := filepath.Abs(filename)
	if err != nil {
		abs = filename
	}
	hash := sha256.Sum256([]byte(abs))
	hashStr := hex.EncodeToString(hash[:16])
	base := filepath.Base(filename)

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dir := filepath.Join(cacheDir, "f4", "tar-indexes")
	_ = os.MkdirAll(dir, 0755)

	return filepath.Join(dir, fmt.Sprintf("%s-%s.index.sqlite", base, hashStr))
}
