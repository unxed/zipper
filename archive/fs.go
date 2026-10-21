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
	"strings"

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
	switch opts.IndexBackend {
	case IndexBackendAuto:
		// leave it to unxed/tar's own build-tag-bound default
	case IndexBackendSQLite:
		fopts = append(fopts, tar.WithFSIndexBackend(tar.IndexBackendSQLite))
	case IndexBackendArcidx:
		fopts = append(fopts, tar.WithFSIndexBackend(tar.IndexBackendArcidx))
	default:
		return nil, fmt.Errorf("zipper: unknown IndexBackend %q (want \"\", %q or %q)", opts.IndexBackend, IndexBackendSQLite, IndexBackendArcidx)
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
	in   *Input
	fsys fs.FS
}

func newFallbackFS(filename string, opts Options) (FileSystem, error) {
	in, err := OpenInput(filename)
	if err != nil {
		return nil, err
	}
	format, _, err := archives.Identify(context.Background(), filename, in)
	if err != nil {
		in.Close()
		return nil, err
	}

	// A split archive exists only as the joined stream of its volumes (see
	// OpenInput). Every other archive is still read by path, as before.
	var stream *io.SectionReader
	if in.Split() {
		stream = in.SectionReader
	}

	var fsys fs.FS
	if opts.Password != "" {
		if passwordFormat, ok := fallbackPasswordFormat(format, opts.Password); ok {
			archiveFS := &archives.ArchiveFS{
				Format:  passwordFormat,
				Context: context.Background(),
			}
			if stream != nil {
				archiveFS.Stream = stream
			} else {
				archiveFS.Path = filename
			}
			fsys = archiveFS
		}
	}
	if fsys == nil {
		if stream != nil {
			fsys, err = archives.FileSystem(context.Background(), filename, stream)
		} else {
			fsys, err = archives.FileSystem(context.Background(), filename, nil)
		}
	}
	if err != nil {
		in.Close()
		return nil, err
	}
	return &fallbackFS{in: in, fsys: fsys}, nil
}

func fallbackPasswordFormat(format archives.Format, password string) (archives.Extractor, bool) {
	switch format := format.(type) {
	case archives.Rar:
		format.Password = password
		return format, true
	case *archives.Rar:
		format.Password = password
		return format, true
	case archives.SevenZip:
		format.Password = password
		return format, true
	case *archives.SevenZip:
		format.Password = password
		return format, true
	default:
		return nil, false
	}
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
	err2 := f.in.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// indexFingerprintEdge is how many bytes from each end of the archive go into
// the fingerprint of its cached index.
const indexFingerprintEdge = 64 << 10

// getDeterministicIndexPath returns where OpenFS keeps the index of a tar
// archive when Options.IndexPath is empty. The name carries the archive's path
// and a fingerprint of its content, so an archive that was appended to or
// rewritten in place finds no index and gets a new one instead of being listed
// from the old. Indexes the same path left under another fingerprint are
// removed. Without a fingerprint (the archive cannot be read) the name has the
// path alone.
func getDeterministicIndexPath(filename string) string {
	abs, err := filepath.Abs(filename)
	if err != nil {
		abs = filename
	}
	hash := sha256.Sum256([]byte(abs))
	prefix := filepath.Base(filename) + "-" + hex.EncodeToString(hash[:16])

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dir := filepath.Join(cacheDir, "f4", "tar-indexes")
	_ = os.MkdirAll(dir, 0755)

	fingerprint, err := archiveFingerprint(filename)
	if err != nil {
		return filepath.Join(dir, prefix+".index.sqlite")
	}
	current := prefix + "-" + fingerprint + ".index.sqlite"
	removeStaleIndexes(dir, prefix, current)
	return filepath.Join(dir, current)
}

// archiveFingerprint hashes the archive's size and its first and last
// indexFingerprintEdge bytes: cheap next to building the index, which reads
// the whole archive, and changed by any append or rewrite.
func archiveFingerprint(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()
	h := sha256.New()
	fmt.Fprintf(h, "%d\x00", size)
	if _, err := io.Copy(h, io.NewSectionReader(f, 0, min(size, indexFingerprintEdge))); err != nil {
		return "", err
	}
	tail := max(size-indexFingerprintEdge, 0)
	if _, err := io.Copy(h, io.NewSectionReader(f, tail, size-tail)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)[:8]), nil
}

// removeStaleIndexes deletes the index files of prefix other than current,
// with the side files SQLite keeps beside them: the plain
// "<prefix>.index.sqlite" and any "<prefix>-<fingerprint>.index.sqlite".
// A file another process holds open may not go away; it is only clutter.
func removeStaleIndexes(dir, prefix, current string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || strings.HasPrefix(name, current) {
			continue
		}
		rest := name[len(prefix):]
		if !strings.HasPrefix(rest, ".index.sqlite") && !(strings.HasPrefix(rest, "-") && strings.Contains(rest, ".index.sqlite")) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}
