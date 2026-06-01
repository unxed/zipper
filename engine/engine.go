package engine

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
	Method      string // "zstd", "deflate", "gzip", "store" и т.д.
}