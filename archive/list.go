package archive

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

// Entry is one entry of an archive as a listing shows it.
type Entry struct {
	Name      string    // slash-separated path inside the archive, without a trailing slash
	Size      int64     // unpacked size in bytes; 0 for a directory
	Packed    int64     // bytes the entry takes in the archive; -1 where the format does not tell it
	Modified  time.Time // modification time
	CRC32     uint32    // CRC-32 of the unpacked data; 0 where the format keeps none
	Method    string    // compression method ("store", "deflate", "zstd", ...); "" where unknown
	Encrypted bool      // the entry's data is encrypted
	IsDir     bool      // a directory, stored or implied by the names of other entries
}

// List returns every entry of the archive, directories included, in the
// order fs.WalkDir visits them (lexical within each directory).
//
// What each format tells:
//   - zip: everything, from the entry headers. Packed, CRC32, Method and
//     Encrypted are those of the stored entry; for a WinZip AES entry Method is
//     the method under the encryption. A directory that only the names of other
//     entries imply has Packed 0 and Method "".
//   - tar (plain or compressed): the archive is one stream, so Packed is -1,
//     CRC32 is 0, Method is the compression of the whole stream ("store",
//     "gzip", "bzip2", "xz", "zstd"; "" when opts has a password, as the stream
//     is then encrypted) and Encrypted is false.
//   - the formats read through the archives package (7z, rar, ...): Name, Size,
//     Modified and IsDir; Packed is -1, CRC32 0 and Method "".
//
// ctx is checked between entries.
func List(ctx context.Context, filename string, opts Options) ([]Entry, error) {
	fsys, err := OpenFS(filename, opts)
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	inner, source := FileSystem(fsys), filename
	if s, ok := inner.(*spoolFS); ok {
		inner, source = s.FileSystem, s.tempFile
	}

	var headers map[string]*zip.File
	tarMethod := ""
	switch f := inner.(type) {
	case *zipFS:
		headers = make(map[string]*zip.File, len(f.zr.File))
		for _, zf := range f.zr.File {
			headers[zipEntryName(zf.Name)] = zf
		}
	case *tarFS:
		// An encrypted stream shows no compression magic of its own.
		if opts.Password == "" {
			if tarMethod, err = tarStreamMethod(source); err != nil {
				return nil, err
			}
		}
	}

	var entries []Entry
	err = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		e := Entry{Name: name, Modified: info.ModTime(), IsDir: d.IsDir(), Packed: -1}
		if !e.IsDir {
			e.Size = info.Size()
		}
		switch {
		case headers != nil:
			e.Packed = 0
			if zf, ok := headers[name]; ok {
				fillZipEntry(&e, &zf.FileHeader)
			}
		case tarMethod != "":
			e.Method = tarMethod
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// zipEntryName gives a zip entry's name the form its fs.FS path has.
func zipEntryName(name string) string {
	p := path.Clean(strings.ReplaceAll(name, `\`, `/`))
	p = strings.TrimPrefix(p, "/")
	for strings.HasPrefix(p, "../") {
		p = p[len("../"):]
	}
	return p
}

// zipAESExtraID is the extra field of a WinZip AES entry: version (2 bytes),
// vendor (2), key strength (1), the method under the encryption (2).
const zipAESExtraID = 0x9901

func fillZipEntry(e *Entry, h *zip.FileHeader) {
	if !e.IsDir {
		e.Size = int64(h.UncompressedSize64)
	}
	e.Packed = int64(h.CompressedSize64)
	e.CRC32 = h.CRC32
	e.Encrypted = h.Flags&0x1 != 0
	method := h.Method
	for extra := h.Extra; len(extra) >= 4; {
		id := binary.LittleEndian.Uint16(extra[0:2])
		size := int(binary.LittleEndian.Uint16(extra[2:4]))
		if len(extra) < 4+size {
			break
		}
		if id == zipAESExtraID && size >= 7 {
			method = binary.LittleEndian.Uint16(extra[4+5 : 4+7])
			e.Encrypted = true
		}
		extra = extra[4+size:]
	}
	if !e.IsDir {
		e.Method = zipMethodName(method)
	}
}

func zipMethodName(m uint16) string {
	switch m {
	case zip.Store:
		return "store"
	case zip.Deflate:
		return "deflate"
	case zip.Deflate64:
		return "deflate64"
	case zip.BZIP2:
		return "bzip2"
	case zip.LZMA:
		return "lzma"
	case zip.ZSTD:
		return "zstd"
	case 95:
		return "xz"
	case 99:
		return "aes"
	}
	return fmt.Sprintf("method %d", m)
}

// tarStreamMethod tells the compression of a tar archive from its first bytes.
func tarStreamMethod(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	m, err := tar.DetectFormat(f)
	if err != nil {
		return "", err
	}
	switch m {
	case tar.GZIP:
		return "gzip", nil
	case tar.BZIP2:
		return "bzip2", nil
	case tar.XZ:
		return "xz", nil
	case tar.ZSTD:
		return "zstd", nil
	}
	return "store", nil
}
