package archive

import (
    "io"
	"fmt"
	"os"

	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

type Updater interface {
	Append(name string, size int64, r io.Reader) error
	Remove(name string) error
	Close() error
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
	// tar.NewUpdater now automatically detects if the archive is compressed (zst, gz)
	// and initializes the correct stream append mode using F4SS shadow streams.
	u, err := tar.NewUpdater(f, tar.APPEND_MODE_OVERWRITE)
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

func (f *fallbackUpdater) Remove(name string) error {
	return fmt.Errorf("archive: in-place updates not supported for fallback formats")
}

func (f *fallbackUpdater) Close() error {
	return nil
}