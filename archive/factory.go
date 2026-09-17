package archive

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultFormat возвращает предпочтительный формат архива для текущей ОС.
func DefaultFormat() string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return ".zip"
	}
	return ".tar.zst"
}

func SpoolStdin() (string, error) {
	f, err := os.CreateTemp("", "zipper-stdin-*.tmp")
	if err != nil {
		return "", err
	}
	defer f.Close()
	// Используем 1МБ буфер для перехвата стандартного ввода (piping)
	if _, err := io.CopyBuffer(f, os.Stdin, make([]byte, 1024*1024)); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// DetectFormat определяет тип движка (zip, tar или fallback) на основе имени файла.
func DetectFormat(filename string) string {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".zip") {
		return "zip"
	}
	// archive.z01, archive.z02, ... are the volumes of a ZIP split archive,
	// whose last volume is the archive.zip beside them. Read as a file of
	// their own they have no central directory at all.
	if isSplitZipVolume(lower) {
		return "zip"
	}
	if strings.HasSuffix(lower, ".tar") || strings.Contains(lower, ".tar.") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".txz") || strings.HasSuffix(lower, ".tbz2") || strings.HasSuffix(lower, ".tzst") {
		return "tar"
	}
	ext := filepath.Ext(lower)
	if ext == ".gz" || ext == ".bz2" || ext == ".xz" || ext == ".zst" || ext == ".rar" || ext == ".7z" {
		return "fallback"
	}

	// Try detecting by content magic bytes
	f, err := os.Open(filename)
	if err == nil {
		defer f.Close()
		buf := make([]byte, 262)
		n, _ := io.ReadFull(f, buf)
		if n >= 4 && string(buf[:4]) == "PK\x03\x04" {
			return "zip"
		}
		// The first volume of a ZIP split archive starts with the
		// spanning marker rather than with a local file header.
		if n >= 4 && string(buf[:4]) == "PK\x07\x08" {
			return "zip"
		}
		// A self-extracting archive is a zip behind an executable, and
		// so is any other file with something in front of the archive.
		// The end of central directory record at the end of the file is
		// what says so, and it is also what the reader goes by.
		if hasZipCentralDirectory(f) {
			return "zip"
		}
		if n >= 262 && string(buf[257:262]) == "ustar" {
			return "tar"
		}
	}

	return ""
}

// NewArchiver возвращает соответствующий Archiver на основе имени файла.
func NewArchiver(filename, chroot string, opts Options) (Archiver, error) {
	fmtType := DetectFormat(filename)
	if fmtType == "zip" {
		return NewZipArchiver(filename, chroot, opts)
	} else if fmtType == "tar" {
		if opts.Method == "" {
			if strings.HasSuffix(filename, ".zst") {
				opts.Method = "zstd"
			} else if strings.HasSuffix(filename, ".gz") || strings.HasSuffix(filename, ".tgz") {
				opts.Method = "gzip"
			} else if strings.HasSuffix(filename, ".xz") || strings.HasSuffix(filename, ".txz") {
				opts.Method = "xz"
			} else if strings.HasSuffix(filename, ".bz2") {
				opts.Method = "bzip2"
			} else {
				opts.Method = "store"
			}
		}
		return NewTarArchiver(filename, chroot, opts)
	}
	return NewFallbackArchiver(filename, chroot, opts)
}

type spoolExtractor struct {
	Extractor
	tempFile string
}

func (s *spoolExtractor) Close() error {
	err := s.Extractor.Close()
	if s.tempFile != "" {
		os.Remove(s.tempFile)
	}
	return err
}

// NewExtractor возвращает соответствующий Extractor на основе имени файла.
func NewExtractor(filename, chroot string, opts Options) (Extractor, error) {
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

	var e Extractor
	var err error
	if fmtType == "zip" {
		e, err = NewZipExtractor(filename, chroot, opts)
	} else if fmtType == "tar" {
		e, err = NewTarExtractor(filename, chroot, opts)
	} else {
		e, err = NewFallbackExtractor(filename, chroot, opts)
	}

	if err != nil {
		if tempFile != "" {
			os.Remove(tempFile)
		}
		return nil, err
	}

	if tempFile != "" {
		return &spoolExtractor{Extractor: e, tempFile: tempFile}, nil
	}
	return e, nil
}

// isSplitZipVolume reports whether name is a ZIP split volume -- .z01, .z02
// and so on, the names WinZip, WinRAR and "zip -s" give the parts before the
// last one. The name is expected in lower case.
func isSplitZipVolume(name string) bool {
	ext := filepath.Ext(name)
	if len(ext) < 4 || !strings.HasPrefix(ext, ".z") {
		return false
	}
	for _, c := range ext[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// zipEndRecordLen is the length of a zip end of central directory record
// without its comment.
const zipEndRecordLen = 22

// hasZipCentralDirectory reports whether the file ends with a zip end of
// central directory record whose central directory really is where the record
// places it. Anything before the archive -- the executable stub of a
// self-extracting archive, for instance -- is left for the zip reader, which
// works out the offset of the entries itself.
func hasZipCentralDirectory(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	size := info.Size()
	if size < zipEndRecordLen {
		return false
	}
	// The record is 22 bytes plus a comment of up to 64 KiB.
	tailLen := int64(zipEndRecordLen + 0xffff)
	if tailLen > size {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := f.ReadAt(tail, size-tailLen); err != nil {
		return false
	}

	for p := len(tail) - zipEndRecordLen; p >= 0; p-- {
		if string(tail[p:p+4]) != "PK\x05\x06" {
			continue
		}
		directorySize := int64(binary.LittleEndian.Uint32(tail[p+12 : p+16]))
		directoryOffset := int64(binary.LittleEndian.Uint32(tail[p+16 : p+20]))
		recordOffset := size - tailLen + int64(p)
		// Where the directory starts, counted from the end of it, and
		// where the record says it starts; a zip behind a stub answers
		// one or the other, depending on whether the tool that wrote it
		// counted the stub in.
		for _, start := range []int64{recordOffset - directorySize, directoryOffset} {
			if start < 0 || start+4 > size {
				continue
			}
			var signature [4]byte
			if _, err := f.ReadAt(signature[:], start); err != nil {
				continue
			}
			if string(signature[:]) == "PK\x01\x02" || (directorySize == 0 && start == recordOffset) {
				return true
			}
		}
	}
	return false
}
