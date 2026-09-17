package archive

import (
	"errors"
	"fmt"
	"io"
	"os"
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
	f *os.File
	u *tar.Updater
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
	return &tarUpdater{f: f, u: u}, nil
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
	switch {
	case fi.IsDir():
		// tar.Updater writes regular file entries only. The files under
		// the directory carry its path and extraction creates it for
		// them; an empty directory has no entry to come back from.
		return nil
	case fi.Mode().IsRegular():
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return t.u.AppendReader(name, fi.Size(), f)
	default:
		return fmt.Errorf("%w: %s is a %s, and tar.Updater writes regular files only", ErrUnsupportedAppend, path, fileKind(fi.Mode()))
	}
}

func (t *tarUpdater) Remove(name string) error {
	return fmt.Errorf("archive: in-place removal is not supported natively for tar")
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
