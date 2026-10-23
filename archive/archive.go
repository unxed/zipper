package archive

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Progresser описывает интерфейс для получения статистики процесса
type Progresser interface {
	Written() (bytes, entries int64)
}

// TestProgressFunc receives cumulative bytes read and files tested.
type TestProgressFunc func(bytes, entries int64)

// Archiver описывает абстрактный интерфейс для создания архивов.
type Archiver interface {
	Archive(ctx context.Context, files map[string]os.FileInfo) error
	Close() error
	Progresser
}

// CurrentFiler reports the archive path currently being extracted.
// It returns an empty string when the extractor has no current file or the
// underlying format engine does not report one.
type CurrentFiler interface {
	CurrentFile() string
}

// ConflictAction determines what extraction does when a target already exists.
type ConflictAction int

const (
	ConflictReplace ConflictAction = iota
	ConflictSkip
	ConflictCancel
)

// Extractor описывает абстрактный интерфейс для распаковки архивов.
type Extractor interface {
	Extract(ctx context.Context) error
	Close() error
	Progresser
}
type stdoutWrapper struct{ *os.File }

func (stdoutWrapper) Close() error { return nil }

// Index backend values for Options.IndexBackend.
const (
	IndexBackendAuto   = ""       // leave the choice to unxed/tar (build-tag bound)
	IndexBackendSQLite = "sqlite" // force the sqlite-backed index
	IndexBackendArcidx = "arcidx" // force the FlatBuffers-backed index
)

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
	NonSolid    bool

	// IndexBackend explicitly picks which tar index implementation OpenFS
	// uses for a tar archive (see unxed/tar's IndexBackend, added alongside
	// its own WithFSIndexBackend): "" (the default) leaves the choice to
	// unxed/tar itself - the sqlite-backed index, unless zipper was built
	// with -tags tarindex_simple, in which case the FlatBuffers-backed one.
	// "sqlite" and "arcidx" force one or the other for this open, regardless
	// of build tag. Ignored for zip/7z/other formats, which have no
	// equivalent index backend choice.
	IndexBackend string

	// Extractor specific
	// Include limits extraction to these paths inside the archive. Selecting a
	// directory includes its entire subtree. An empty slice extracts everything.
	Include []string
	// OnConflict is called with the archive path when extraction would replace
	// an existing target. It is ignored when nil.
	OnConflict func(name string) ConflictAction

	RecoveryPct int

	KeepOldFiles   bool
	KeepNewerFiles bool
	KeepBroken     bool
	Sparse         bool
	Tolerant       bool
	SafeWrites     bool
	UnlinkFirst    bool
	NumericOwner   bool

	TorrentZip         bool
	NoPlatformMetadata bool
	NoTimes            bool
	StripComponents    int
	MaxFileSize        int64
	MaxRatio           int64
	RecoveryExternal   bool
	Lock               bool
	PathMapping        map[string]string
}

func normalizeInclude(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(paths))
	cleaned := make([]string, 0, len(paths))
	for _, name := range paths {
		if name == "" || strings.Contains(name, "\\") {
			return nil, fmt.Errorf("archive: invalid include path %q: use a non-empty slash-separated archive path", name)
		}
		clean := path.Clean(name)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) ||
			filepath.IsAbs(filepath.FromSlash(clean)) || filepath.VolumeName(filepath.FromSlash(clean)) != "" || hasDrivePrefix(clean) {
			return nil, fmt.Errorf("archive: invalid include path %q: path must stay inside the archive", name)
		}
		if _, ok := seen[clean]; !ok {
			seen[clean] = struct{}{}
			cleaned = append(cleaned, clean)
		}
	}
	return cleaned, nil
}

func includedPath(name string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) ||
		filepath.IsAbs(filepath.FromSlash(clean)) || filepath.VolumeName(filepath.FromSlash(clean)) != "" || hasDrivePrefix(clean) {
		return false
	}
	for _, include := range includes {
		if clean == include || strings.HasPrefix(clean, include+"/") {
			return true
		}
	}
	return false
}

func hasDrivePrefix(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
}
