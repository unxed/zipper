# Unified Archive Library

This package is a high-level, cross-platform archive abstraction library. It wraps `unxed/zip` and `unxed/tar` to leverage high-performance and high-fidelity features (such as parallel writing/extracting, NTFS ACLs, UNIX xattrs, and seekable solid archives) while gracefully falling back to standard Go implementations for other formats (via `mholt/archives`).

## Core API Interfaces

### 1. Options
Configuration struct for both archiving and extraction operations.
```go
type Options struct {
	Concurrency    int
	Xattrs         bool
	Solid          bool
	Method         string // "zstd", "deflate", "gzip", "store", "lzma", "bzip2", "xz"
	Password       string
	EncryptCD      bool
	SeekChunkSize  uint32
	SeekContinuous bool
	Incremental    bool
	IndexPath      string
	EmbeddedIdx    bool

	// Extractor specific
	KeepOldFiles   bool
	KeepNewerFiles bool
	KeepBroken     bool
	Sparse         bool
	Tolerant       bool
	SafeWrites     bool
	UnlinkFirst    bool
	NumericOwner   bool
}
```

### 2. Archiver & Extractor
Used for batch file packing and unpacking.
```go
type Archiver interface {
	Archive(ctx context.Context, files map[string]os.FileInfo) error
	Close() error
}

type Extractor interface {
	Extract(ctx context.Context) error
	Close() error
}
```

### 3. FileSystem
Provides read-only VFS access (`fs.FS`) to zip, tar, and fallback formats.
```go
type FileSystem interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS
	io.Closer
}
```

### 4. Updater
Enables in-place modifications (append/remove) on supported formats.
```go
type Updater interface {
	Append(name string, size int64, r io.Reader) error
	Remove(name string) error
	Close() error
}
```

---

## Code Examples

### 1. File Extraction
```go
package main

import (
	"context"
	"log"
	"github.com/unxed/zipper/archive"
)

func main() {
	opts := archive.Options{Xattrs: true, SafeWrites: true}
	e, err := archive.NewExtractor("my_archive.zip", "./output_dir", opts)
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()

	if err := e.Extract(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```

### 2. Reading files via fs.FS (Virtual File System)
```go
package main

import (
	"fmt"
	"io"
	"log"
	"github.com/unxed/zipper/archive"
)

func main() {
	fsys, err := archive.OpenFS("archive.tar.zst", archive.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer fsys.Close()

	f, err := fsys.Open("nested/file.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	data, _ := io.ReadAll(f)
	fmt.Println(string(data))
}
```

### 3. In-place Append
```go
package main

import (
	"log"
	"strings"
	"github.com/unxed/zipper/archive"
)

func main() {
	upd, err := archive.NewUpdater("existing.zip", archive.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer upd.Close()

	content := "updated data stream"
	r := strings.NewReader(content)

	err = upd.Append("config.txt", int64(len(content)), r)
	if err != nil {
		log.Fatal(err)
	}
}
```
