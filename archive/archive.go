package archive

import (
	"context"
	"os"
)

// Progresser описывает интерфейс для получения статистики процесса
type Progresser interface {
	Written() (bytes, entries int64)
}

// Archiver описывает абстрактный интерфейс для создания архивов.
type Archiver interface {
	Archive(ctx context.Context, files map[string]os.FileInfo) error
	Close() error
	Progresser
}

// Extractor описывает абстрактный интерфейс для распаковки архивов.
type Extractor interface {
	Extract(ctx context.Context) error
	Close() error
	Progresser
}
type stdoutWrapper struct{ *os.File }

func (stdoutWrapper) Close() error { return nil }

// Options содержит унифицированные параметры как для zip, так и для tar.
type Options struct {
	Concurrency int
	Xattrs      bool
	Solid       bool
	Method      string // "zstd", "deflate", "gzip", "store", "lzma", "bzip2", "xz"
	Level       int    // Compression level (1-9)

	Password       string
	EncryptCD      bool
	SeekChunkSize  uint32
	SeekContinuous bool
	SplitSize      int64

	// Archiver specific
	Incremental bool
	IndexPath   string
	EmbeddedIdx bool

	// Extractor specific
	RecoveryPct int

	KeepOldFiles   bool
	KeepNewerFiles bool
	KeepBroken     bool
	Sparse         bool
	Tolerant       bool
	SafeWrites     bool
	UnlinkFirst    bool
	NumericOwner   bool

	TorrentZip     bool
	NoPlatformMetadata bool
	NoTimes            bool
	StripComponents    int
	MaxFileSize        int64
	MaxRatio           int64
	RecoveryExternal   bool
	Lock               bool
}