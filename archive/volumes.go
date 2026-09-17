package archive

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Input is an archive opened for reading: one file, or every volume of a
// split archive read as one continuous stream. It reads, reads at an offset
// and seeks like the *io.SectionReader it embeds, and Close closes the files.
type Input struct {
	*io.SectionReader
	files []*os.File
}

// OpenInput opens filename as archive input.
//
// A name ending in ".001" is the first volume of a split archive, the naming
// 7-Zip uses for its "-v" volumes and the rule sevenzip.OpenReader documents:
// ".002", ".003" and so on are opened from the same directory up to the first
// missing number, and read after it as one stream. Such volumes are a plain
// byte split of one archive, so an archive's own offsets span all of them;
// handing a reader only the first volume fails at the header stored at the end
// ("sevenzip: error reading header id: EOF", f4 issue #1179).
func OpenInput(filename string) (*Input, error) {
	first, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	files := []*os.File{first}
	closeAll := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}

	if filepath.Ext(filename) == ".001" {
		stem := strings.TrimSuffix(filename, ".001")
		for volume := 2; ; volume++ {
			f, err := os.Open(fmt.Sprintf("%s.%03d", stem, volume))
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					break
				}
				closeAll()
				return nil, err
			}
			files = append(files, f)
		}
	}

	starts := make([]int64, 1, len(files)+1)
	for _, f := range files {
		info, err := f.Stat()
		if err != nil {
			closeAll()
			return nil, err
		}
		starts = append(starts, starts[len(starts)-1]+info.Size())
	}

	var reader io.ReaderAt = first
	if len(files) > 1 {
		reader = &volumeReaderAt{files: files, starts: starts}
	}
	return &Input{
		SectionReader: io.NewSectionReader(reader, 0, starts[len(starts)-1]),
		files:         files,
	}, nil
}

// Split reports whether the input spans more than one volume.
func (in *Input) Split() bool { return len(in.files) > 1 }

// Volumes lists the files the input reads, in order.
func (in *Input) Volumes() []string {
	names := make([]string, len(in.files))
	for i, f := range in.files {
		names[i] = f.Name()
	}
	return names
}

// Close closes every volume.
func (in *Input) Close() error {
	var errs []error
	for _, f := range in.files {
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// volumeReaderAt reads a sequence of files as one. starts[i] is the offset of
// files[i] within the whole; the last element is the total size.
type volumeReaderAt struct {
	files  []*os.File
	starts []int64
}

func (v *volumeReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("archive: negative volume offset")
	}
	total := v.starts[len(v.starts)-1]
	if off >= total {
		return 0, io.EOF
	}
	index := sort.Search(len(v.files), func(i int) bool { return v.starts[i+1] > off })
	n := 0
	for n < len(p) && index < len(v.files) {
		partOffset := off + int64(n) - v.starts[index]
		chunk := p[n:]
		if rest := v.starts[index+1] - v.starts[index] - partOffset; int64(len(chunk)) > rest {
			chunk = chunk[:rest]
		}
		read, err := v.files[index].ReadAt(chunk, partOffset)
		n += read
		if read < len(chunk) {
			if err == nil || errors.Is(err, io.EOF) {
				// The volume is shorter than it was when opened.
				err = io.ErrUnexpectedEOF
			}
			return n, err
		}
		index++
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
