package archive

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mholt/archives"
	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

type FileSystem interface {
	fs.FS
	fs.ReadDirFS
	fs.StatFS
	io.Closer
}

func logToFile(format string, args ...any) {
	f, err := os.OpenFile("/tmp/zipper-archive.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		fmt.Fprintf(f, "[%s] "+format+"\n", append([]any{time.Now().Format("15:04:05.000")}, args...)...)
	}
}

func OpenFS(filename string, opts Options) (FileSystem, error) {
	logToFile("OpenFS called for filename: %q, method: %q", filename, opts.Method)
	fmtType := DetectFormat(filename)
	logToFile("OpenFS detected format: %q", fmtType)
	if fmtType == "zip" {
		return newZipFS(filename, opts)
	} else if fmtType == "tar" {
		return newTarFS(filename, opts)
	}
	return newFallbackFS(filename, opts)
}

func cleanPath(name string) string {
	name = strings.ReplaceAll(name, `\`, `/`)
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "." {
		return "."
	}
	return name
}

type zipFS struct {
	zr *zip.ReadCloser
}

func newZipFS(filename string, opts Options) (FileSystem, error) {
	logToFile("newZipFS opening filename: %q", filename)
	zr, err := zip.OpenReaderWithPassword(filename, opts.Password)
	if err != nil {
		logToFile("newZipFS open failed: %v", err)
		return nil, err
	}
	logToFile("newZipFS successfully opened")
	return &zipFS{zr: zr}, nil
}

func (z *zipFS) Open(name string) (fs.File, error) {
	cleaned := cleanPath(name)
	logToFile("zipFS.Open name: %q, cleaned: %q", name, cleaned)
	f, err := z.zr.Open(cleaned)
	if err != nil {
		logToFile("zipFS.Open error: %v", err)
	}
	return f, err
}

func (z *zipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	cleaned := cleanPath(name)
	logToFile("zipFS.ReadDir name: %q, cleaned: %q", name, cleaned)
	entries, err := fs.ReadDir(z.zr, cleaned)
	if err != nil {
		logToFile("zipFS.ReadDir error: %v", err)
	} else {
		logToFile("zipFS.ReadDir returned %d entries", len(entries))
	}
	return entries, err
}

func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
	cleaned := cleanPath(name)
	logToFile("zipFS.Stat name: %q, cleaned: %q", name, cleaned)
	info, err := fs.Stat(z.zr, cleaned)
	if err != nil {
		logToFile("zipFS.Stat error: %v", err)
	}
	return info, err
}

func (z *zipFS) Close() error {
	return z.zr.Close()
}

type tarFS struct {
	tfs *tar.TarFS
}

func newTarFS(filename string, opts Options) (FileSystem, error) {
	logToFile("newTarFS called for filename: %q, opts.IndexPath: %q", filename, opts.IndexPath)
	tfs, err := tar.NewFS(filename, opts.IndexPath)
	if err != nil {
		logToFile("newTarFS: tar.NewFS failed: %v", err)
		return nil, err
	}
	logToFile("newTarFS: tar.NewFS successfully initialized TarFS pointer: %p", tfs)

	// Выполним диагностический запрос прямо к БД индексов для замера кол-ва файлов
	if tfs != nil && tfs.IndexPath != "" {
		db, errDb := sql.Open("sqlite", tfs.IndexPath)
		if errDb == nil {
			defer db.Close()
			var count int
			errQuery := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&count)
			logToFile("newTarFS: DIAGNOSTIC index database file count: %d, queryErr: %v", count, errQuery)

			// Выведем топ-10 записей для сверки структуры путей в БД
			rows, errRows := db.Query("SELECT path, name, isgenerated FROM files LIMIT 10")
			if errRows == nil {
				for rows.Next() {
					var p, n string
					var isGen bool
					rows.Scan(&p, &n, &isGen)
					logToFile("newTarFS: DIAGNOSTIC DB ROW: path=%q, name=%q, isgenerated=%v", p, n, isGen)
				}
				rows.Close()
			} else {
				logToFile("newTarFS: DIAGNOSTIC DB rows query failed: %v", errRows)
			}
		} else {
			logToFile("newTarFS: DIAGNOSTIC DB connection failed: %v", errDb)
		}
	} else {
		logToFile("newTarFS: WARNING: tfs.IndexPath is empty, DB query skipped!")
	}

	return &tarFS{tfs: tfs}, nil
}

func (t *tarFS) Open(name string) (fs.File, error) {
	cleaned := cleanPath(name)
	logToFile("tarFS.Open name: %q, cleaned: %q", name, cleaned)
	f, err := t.tfs.Open(cleaned)
	if err != nil {
		logToFile("tarFS.Open error: %v", err)
	}
	return f, err
}

func (t *tarFS) ReadDir(name string) ([]fs.DirEntry, error) {
	cleaned := cleanPath(name)
	logToFile("tarFS.ReadDir name: %q, cleaned: %q", name, cleaned)
	entries, err := fs.ReadDir(t.tfs, cleaned)
	if err != nil {
		logToFile("tarFS.ReadDir error: %v", err)
	} else {
		logToFile("tarFS.ReadDir returned %d entries", len(entries))
		for idx, entry := range entries {
			logToFile("tarFS.ReadDir ENTRY [%d]: name=%q, isDir=%v", idx, entry.Name(), entry.IsDir())
		}
	}
	return entries, err
}

func (t *tarFS) Stat(name string) (fs.FileInfo, error) {
	cleaned := cleanPath(name)
	logToFile("tarFS.Stat name: %q, cleaned: %q", name, cleaned)
	info, err := fs.Stat(t.tfs, cleaned)
	if err != nil {
		logToFile("tarFS.Stat error: %v", err)
	}
	return info, err
}

func (t *tarFS) Close() error {
	return t.tfs.Close()
}

type fallbackFS struct {
	f    *os.File
	fsys fs.FS
}

func newFallbackFS(filename string, opts Options) (FileSystem, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	_, _, err = archives.Identify(context.Background(), filename, f)
	if err != nil {
		f.Close()
		return nil, err
	}

	fsys, err := archives.FileSystem(context.Background(), filename, nil)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &fallbackFS{f: f, fsys: fsys}, nil
}

func (f *fallbackFS) Open(name string) (fs.File, error) {
	return f.fsys.Open(cleanPath(name))
}

func (f *fallbackFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.fsys, cleanPath(name))
}

func (f *fallbackFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.fsys, cleanPath(name))
}

func (f *fallbackFS) Close() error {
	var err1 error
	if closer, ok := f.fsys.(io.Closer); ok {
		err1 = closer.Close()
	}
	err2 := f.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}