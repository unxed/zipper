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
	IndexBackend   string // "" (auto), "sqlite", or "arcidx" - see unxed/tar's IndexBackend
	EmbeddedIdx    bool

	// Extractor specific
	Include        []string
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

type CurrentFiler interface {
	CurrentFile() string
}

type ConflictAction int
const (
	ConflictReplace ConflictAction = iota
	ConflictSkip
	ConflictCancel
)

type Options struct {
	// ...
	OnConflict func(name string) ConflictAction
}
```
Extractors implement `CurrentFiler`; it reports the path currently being extracted, or an empty string when no current path is available.
When `Options.OnConflict` is set, it is called for each archive path whose destination already exists. Return `ConflictReplace` to replace it, `ConflictSkip` to leave it unchanged, or `ConflictCancel` to stop extraction with `context.Canceled`. With a callback set, extraction uses the fallback extractor for all supported formats.

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

### 5. List
Lists every entry of an archive with what its format tells about it, for an archiver window or a detailed listing.
```go
type Entry struct {
	Name      string    // path inside the archive, no trailing slash
	Size      int64     // unpacked size; 0 for a directory
	Packed    int64     // size in the archive; -1 where the format does not tell it
	Modified  time.Time
	CRC32     uint32    // 0 where the format keeps none
	Method    string    // "store", "deflate", "zstd", ...; "" where unknown
	Encrypted bool
	IsDir     bool
}

func List(ctx context.Context, filename string, opts Options) ([]Entry, error)
```
zip fills every field from the entry headers. tar is one stream, so its entries have `Packed` -1, `CRC32` 0 and `Method` set to the compression of the whole archive. 7z, rar and the other formats read through `archives` give the name, size, time and directory flag.

### 6. Test

Read every file in an archive to EOF without extracting it. ZIP data is read through its CRC-checking reader; tar and fallback formats are streamed through `archives`. Per-member failures are returned with the member name; archive-open and format-identification failures are returned as `error`.

```go
type TestError struct {
	Name string
	Err  error
}

type TestProgressFunc func(bytes, entries int64)

func Test(ctx context.Context, filename string, opts Options, progress TestProgressFunc) ([]TestError, error)
```

The optional progress callback receives cumulative uncompressed bytes read and files tested.

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

To extract only selected archive paths, pass them in `Options.Include`. Paths use `/` regardless of the host OS; selecting a directory includes that directory and its entire subtree. An empty `Include` extracts the whole archive.

```go
opts := archive.Options{Include: []string{"docs/readme.txt", "images"}}
e, err := archive.NewExtractor("backup.zip", "./output_dir", opts)
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
