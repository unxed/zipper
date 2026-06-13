package archive

import (
	"context"
	"os"
)

// Archiver описывает абстрактный интерфейс для создания архивов.
type Archiver interface {
	Archive(ctx context.Context, files map[string]os.FileInfo) error
	Close() error
}

// Extractor описывает абстрактный интерфейс для распаковки архивов.
type Extractor interface {
	Extract(ctx context.Context) error
	Close() error
}

// Options содержит унифицированные параметры как для zip, так и для tar.
type Options struct {
	Concurrency int
	Xattrs      bool
	Solid       bool
	Method      string // "zstd", "deflate", "gzip", "store", "lzma", "bzip2", "xz"

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
}