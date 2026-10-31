package archive

import (
	stdtar "archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

type Updater interface {
	Append(name string, size int64, r io.Reader) error
	// AppendFile adds what is at path on disk to the archive under name:
	// a regular file with its contents, a directory as an entry of its
	// own, a symbolic link as a link, each with the mode and modification
	// time fi reports. It does not descend into a directory; the caller
	// walks the tree. An entry the format's updater cannot represent is
	// refused with an error that wraps ErrUnsupportedAppend.
	AppendFile(name, path string, fi os.FileInfo) error
	Remove(name string) error
	Close() error
}

// ErrUnsupportedAppend is wrapped by the error AppendFile returns for a file
// the archive's updater has no entry for, such as a FIFO or a device node.
var ErrUnsupportedAppend = errors.New("archive: the updater has no entry for this kind of file")

func fileKind(m os.FileMode) string {
	switch {
	case m&os.ModeSymlink != 0:
		return "symbolic link"
	case m&os.ModeNamedPipe != 0:
		return "named pipe"
	case m&os.ModeSocket != 0:
		return "socket"
	case m&os.ModeCharDevice != 0:
		return "character device"
	case m&os.ModeDevice != 0:
		return "block device"
	default:
		return "irregular file"
	}
}

func NewUpdater(filename string, opts Options) (Updater, error) {
	if filename == "-" {
		return nil, fmt.Errorf("archive: in-place updates not supported for standard input/output")
	}

	fmtType := DetectFormat(filename)
	if fmtType == "zip" {
		return newZipUpdater(filename, opts)
	} else if fmtType == "tar" {
		return newTarUpdater(filename, opts)
	}
	return newFallbackUpdater(filename, opts)
}

type zipUpdater struct {
	f *os.File
	u *zip.Updater
}

func newZipUpdater(filename string, opts Options) (Updater, error) {
	f, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	u, err := zip.NewUpdater(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &zipUpdater{f: f, u: u}, nil
}

func (z *zipUpdater) Append(name string, size int64, r io.Reader) error {
	w, err := z.u.Append(name, zip.APPEND_MODE_OVERWRITE)
	if err != nil {
		return err
	}
	if r != nil {
		_, err = io.CopyBuffer(w, r, make([]byte, 1024*1024))
	}
	return err
}

func (z *zipUpdater) AppendFile(name, path string, fi os.FileInfo) error {
	fh, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	fh.Name = name
	switch {
	case fi.IsDir():
		if !strings.HasSuffix(fh.Name, "/") {
			fh.Name += "/"
		}
		_, err = z.u.AppendHeader(fh, zip.APPEND_MODE_OVERWRITE)
		return err
	case fi.Mode()&os.ModeSymlink != 0:
		// The link's target is its body, stored, as the archiver writes it.
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		fh.Method = zip.Store
		w, err := z.u.AppendHeader(fh, zip.APPEND_MODE_OVERWRITE)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, target)
		return err
	case fi.Mode().IsRegular():
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fh.Method = zip.Deflate
		w, err := z.u.AppendHeader(fh, zip.APPEND_MODE_OVERWRITE)
		if err != nil {
			return err
		}
		_, err = io.CopyBuffer(w, f, make([]byte, 1024*1024))
		return err
	default:
		return fmt.Errorf("%w: %s is a %s", ErrUnsupportedAppend, path, fileKind(fi.Mode()))
	}
}

func (z *zipUpdater) Remove(name string) error {
	entries := z.u.Entries()
	for i, e := range entries {
		if e.Name == name {
			_, err := z.u.RemoveFile(i)
			return err
		}
	}
	return os.ErrNotExist
}

func (z *zipUpdater) Close() error {
	err1 := z.u.Close()
	err2 := z.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

type tarUpdater struct {
	f        *os.File
	u        *tar.Updater
	filename string
	opts     Options
}

func newTarUpdater(filename string, opts Options) (Updater, error) {
	f, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	var topts []tar.WriterOption
	if opts.Level != 0 {
		topts = append(topts, tar.WithWriterLevel(opts.Level))
	}
	// tar.NewUpdater now automatically detects if the archive is compressed (zst, gz)
	// and initializes the correct stream append mode using F4SS shadow streams.
	u, err := tar.NewUpdater(f, tar.APPEND_MODE_OVERWRITE, topts...)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &tarUpdater{f: f, u: u, filename: filename, opts: opts}, nil
}

func (t *tarUpdater) Append(name string, size int64, r io.Reader) error {
	var data []byte
	if r != nil {
		var err error
		data, err = io.ReadAll(r)
		if err != nil {
			return err
		}
	}
	return t.u.Append(name, size, data)
}

func (t *tarUpdater) AppendFile(name, path string, fi os.FileInfo) error {
	var link string
	if fi.Mode()&os.ModeSymlink != 0 {
		var err error
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}
	// FileInfoHeader gives the entry its type, mode, times and owner; it
	// refuses what tar has no type for, a socket.
	hdr, err := stdtar.FileInfoHeader(fi, link)
	if err != nil {
		return fmt.Errorf("%w: %s is a %s: %v", ErrUnsupportedAppend, path, fileKind(fi.Mode()), err)
	}
	hdr.Name = name
	if fi.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
		hdr.Name += "/"
	}
	if !fi.Mode().IsRegular() {
		return t.u.AppendHeader(hdr, nil)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.u.AppendHeader(hdr, f)
}

// Remove removes an entry or directory subtree from the tar archive.
// Because tar does not natively support in-place removal, this performs a full
// repack of the archive to a temporary file, with cost proportional to the archive size.
// An entry is removed if its path matches name exactly or starts with the directory prefix name/.
// Removal is only supported for TAR wrappers whose compression can be recreated
// (.tar, .tar.zst, .tzst, .tar.gz, .tgz, .tar.xz, .txz). For other wrappers
// (such as .tar.bz2, .tbz2, .tar.lzma), Remove returns an error before touching the
// original archive to avoid altering its compression format.
func (t *tarUpdater) Remove(name string) error {
	tempOpts := t.opts
	tempOpts.RecoveryPct = 0
	if tempOpts.Method == "" {
		lower := strings.ToLower(t.filename)
		if strings.HasSuffix(lower, ".zst") || strings.HasSuffix(lower, ".tzst") {
			tempOpts.Method = "zstd"
		} else if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
			tempOpts.Method = "gzip"
		} else if strings.HasSuffix(lower, ".xz") || strings.HasSuffix(lower, ".txz") {
			tempOpts.Method = "xz"
		} else if strings.HasSuffix(lower, ".tar") {
			tempOpts.Method = "store"
		} else {
			ext := filepath.Ext(t.filename)
			if ext == "" {
				ext = filepath.Base(t.filename)
			}
			return fmt.Errorf("archive: removal is not supported for %s", ext)
		}
	} else {
		switch tempOpts.Method {
		case "zstd", "gzip", "xz", "store":
		default:
			return fmt.Errorf("archive: removal is not supported for %s", tempOpts.Method)
		}
	}

	// In-place removal is not supported natively for tar.
	// We repack the archive to a temporary file, skipping the entry and its subtree,
	// then replace the original.
	if err := t.u.Close(); err != nil {
		t.f.Close()
		return err
	}
	if err := t.f.Close(); err != nil {
		return err
	}

	reopen := func() error {
		f, err := os.OpenFile(t.filename, os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		var topts []tar.WriterOption
		if t.opts.Level != 0 {
			topts = append(topts, tar.WithWriterLevel(t.opts.Level))
		}
		u, err := tar.NewUpdater(f, tar.APPEND_MODE_OVERWRITE, topts...)
		if err != nil {
			f.Close()
			return err
		}
		t.f = f
		t.u = u
		return nil
	}

	tempName := t.filename + ".tmp"
	defer os.Remove(tempName)

	emptyArchiver, err := NewTarArchiver(tempName, "", tempOpts)
	if err != nil {
		reopen()
		return fmt.Errorf("archive: failed to create temporary archive: %w", err)
	}
	if err := emptyArchiver.Archive(context.Background(), nil); err != nil {
		emptyArchiver.Close()
		reopen()
		return fmt.Errorf("archive: failed to initialize temporary archive: %w", err)
	}
	if err := emptyArchiver.Close(); err != nil {
		reopen()
		return fmt.Errorf("archive: failed to close temporary archive: %w", err)
	}

	fsys, err := OpenFS(t.filename, t.opts)
	if err != nil {
		reopen()
		return fmt.Errorf("archive: failed to open original archive for reading: %w", err)
	}

	tf, err := os.OpenFile(tempName, os.O_RDWR, 0644)
	if err != nil {
		fsys.Close()
		reopen()
		return fmt.Errorf("archive: failed to open temporary archive for appending: %w", err)
	}

	var topts []tar.WriterOption
	if t.opts.Level != 0 {
		topts = append(topts, tar.WithWriterLevel(t.opts.Level))
	}
	newU, err := tar.NewUpdater(tf, tar.APPEND_MODE_OVERWRITE, topts...)
	if err != nil {
		tf.Close()
		fsys.Close()
		reopen()
		return fmt.Errorf("archive: failed to initialize temporary updater: %w", err)
	}

	matchName := name
	if strings.HasSuffix(matchName, "/") {
		matchName = matchName[:len(matchName)-1]
	}
	removedAny := false

	copyErr := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if path == matchName || strings.HasPrefix(path, matchName+"/") {
			removedAny = true
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		var hdr *stdtar.Header
		if sys := info.Sys(); sys != nil {
			if h, ok := sys.(*stdtar.Header); ok {
				hdr = h
			}
		}
		if hdr == nil {
			hdr, err = stdtar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = path
			if d.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}
		}

		var r io.Reader
		if !d.IsDir() && info.Mode().IsRegular() {
			f, err := fsys.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			r = f
		}
		return newU.AppendHeader(hdr, r)
	})

	fsys.Close()

	if copyErr != nil {
		newU.Close()
		tf.Close()
		reopen()
		return copyErr
	}
	if !removedAny {
		newU.Close()
		tf.Close()
		reopen()
		return os.ErrNotExist
	}

	if err := newU.Close(); err != nil {
		tf.Close()
		reopen()
		return err
	}
	if err := tf.Close(); err != nil {
		reopen()
		return err
	}

	if err := os.Rename(tempName, t.filename); err != nil {
		reopen()
		return err
	}

	return reopen()
}

func (t *tarUpdater) Close() error {
	err1 := t.u.Close()
	err2 := t.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

type fallbackUpdater struct {
	filename string
	opts     Options
}

func newFallbackUpdater(filename string, opts Options) (Updater, error) {
	return &fallbackUpdater{filename: filename, opts: opts}, nil
}

func (f *fallbackUpdater) Append(name string, size int64, r io.Reader) error {
	return fmt.Errorf("archive: in-place updates not supported for fallback formats")
}

func (f *fallbackUpdater) AppendFile(name, path string, fi os.FileInfo) error {
	return fmt.Errorf("archive: in-place updates not supported for fallback formats")
}

func (f *fallbackUpdater) Remove(name string) error {
	return fmt.Errorf("archive: in-place updates not supported for fallback formats")
}

func (f *fallbackUpdater) Close() error {
	return nil
}
