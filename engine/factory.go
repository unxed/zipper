package engine

import (
	"errors"
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

// DetectFormat определяет тип движка (zip или tar) на основе имени файла.
func DetectFormat(filename string) string {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".zip") {
		return "zip"
	}
	if strings.HasSuffix(lower, ".tar") || strings.Contains(lower, ".tar.") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".txz") {
		return "tar"
	}
	return ""
}

// NewArchiver возвращает соответствующий Archiver на основе имени файла.
func NewArchiver(filename, chroot string, opts Options) (Archiver, error) {
	fmt := DetectFormat(filename)
	if fmt == "zip" {
		return NewZipArchiver(filename, chroot, opts)
	} else if fmt == "tar" {
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
	return nil, errors.New("unsupported archive format")
}

// NewExtractor возвращает соответствующий Extractor на основе имени файла.
func NewExtractor(filename, chroot string, opts Options) (Extractor, error) {
	fmt := DetectFormat(filename)
	if fmt == "zip" {
		return NewZipExtractor(filename, chroot, opts)
	} else if fmt == "tar" {
		return NewTarExtractor(filename, chroot, opts)
	}
	return nil, errors.New("unsupported archive format")
}