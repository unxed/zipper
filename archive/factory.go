package archive

import (
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
