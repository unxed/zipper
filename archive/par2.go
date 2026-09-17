package archive

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/unxed/par2"
	"github.com/unxed/tar"
	"github.com/unxed/zip"
)

type VolumeReaderRW interface {
	ReadAt(p []byte, off int64) (n int, err error)
	WriteAt(p []byte, off int64) (n int, err error)
}

type sectionRepairTarget struct {
	target VolumeReaderRW
	size   int64
}

func (s *sectionRepairTarget) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= s.size {
		return 0, io.EOF
	}
	if off+int64(len(p)) > s.size {
		p = p[:s.size-off]
	}
	return s.target.ReadAt(p, off)
}

func (s *sectionRepairTarget) WriteAt(p []byte, off int64) (n int, err error) {
	if off >= s.size {
		return 0, fmt.Errorf("write out of bounds")
	}
	if off+int64(len(p)) > s.size {
		p = p[:s.size-off]
	}
	return s.target.WriteAt(p, off)
}

// RepairZipArchive извлекает скрытый по стандарту SOZip файл .recovery.par2
// и чинит архив in-place (в том числе многотомный) через unxed/par2.
func RepairZipArchive(filename string) error {
	mvr, totalSize, err := zip.OpenMultiVolume(filename, os.O_RDWR)
	if err != nil {
		return err
	}
	defer mvr.Close()

	var parOffset int64
	var parSize int64
	buf := make([]byte, 30)

	for off := int64(0); off < totalSize-30; {
		if _, err := mvr.ReadAt(buf[:4], off); err != nil {
			break
		}
		if binary.LittleEndian.Uint32(buf[:4]) == 0x04034b50 { // LFH Signature
			if _, err := mvr.ReadAt(buf[4:], off+4); err != nil {
				break
			}
			nlen := binary.LittleEndian.Uint16(buf[26:28])
			elen := binary.LittleEndian.Uint16(buf[28:30])
			nameBuf := make([]byte, nlen)
			if _, err := mvr.ReadAt(nameBuf, off+30); err != nil {
				break
			}
			if string(nameBuf) == ".recovery.par2" {
				parOffset = off
				parSize = int64(binary.LittleEndian.Uint32(buf[18:22]))
				break
			}
			off += 30 + int64(nlen) + int64(elen) + int64(binary.LittleEndian.Uint32(buf[18:22]))
		} else {
			off++
		}
	}

	if parOffset == 0 {
		externalParPath := filename + ".par2"
		parData, err := os.ReadFile(externalParPath)
		if err == nil && len(parData) > 0 {
			srt := &sectionRepairTarget{target: mvr, size: totalSize}
			return par2.RepairTargetData(srt, parData)
		}
		return fmt.Errorf("embedded recovery record (.recovery.par2) or external .par2 sidecar not found")
	}

	// Стримим parity-данные во временный буфер
	var parData bytes.Buffer
	var hdr [30]byte
	if _, err := mvr.ReadAt(hdr[:], parOffset); err != nil {
		return fmt.Errorf("failed to read recovery entry header: %w", err)
	}
	dataStart := parOffset + 30 + int64(binary.LittleEndian.Uint16(hdr[26:28])) + int64(binary.LittleEndian.Uint16(hdr[28:30]))
	sr := io.NewSectionReader(mvr, dataStart, parSize)
	if _, err := io.CopyBuffer(&parData, sr, make([]byte, 1024*1024)); err != nil {
		return fmt.Errorf("failed to read recovery payload: %w", err)
	}

	_, fileDesc, _, _, err := par2.ParsePackets(parData.Bytes())
	if err != nil || fileDesc == nil {
		return fmt.Errorf("recovery payload is not a PAR2 stream: %v", err)
	}
	covered := int64(fileDesc.Length)

	// The recovery data covers the archive up to the recovery entry and,
	// in archives written since the entry was moved in front of the central
	// directory, the directory that follows the entry's body too. Before,
	// the entry followed the directory, and what it covered ended where it
	// began.
	if covered <= parOffset {
		srt := &sectionRepairTarget{target: mvr, size: parOffset}
		return par2.RepairTargetData(srt, parData.Bytes())
	}
	cdStart := dataStart + parSize
	if cdStart+(covered-parOffset) > totalSize {
		return fmt.Errorf("recovery data covers %d bytes, more than the archive holds", covered)
	}
	target := &rangesRepairTarget{target: mvr, ranges: []repairRange{
		{virtual: 0, file: 0, length: parOffset},
		{virtual: parOffset, file: cdStart, length: covered - parOffset},
	}}
	return par2.RepairTargetData(target, parData.Bytes())
}

// repairRange maps length bytes of the stream the recovery data covers,
// starting at virtual, onto the archive starting at file.
type repairRange struct {
	virtual, file, length int64
}

// rangesRepairTarget presents ranges of an archive as the one contiguous
// stream the recovery data was computed over.
type rangesRepairTarget struct {
	target VolumeReaderRW
	ranges []repairRange
}

func (t *rangesRepairTarget) size() int64 {
	last := t.ranges[len(t.ranges)-1]
	return last.virtual + last.length
}

func (t *rangesRepairTarget) ReadAt(p []byte, off int64) (int, error) {
	n, err := t.each(p, off, t.target.ReadAt)
	if err == nil && n < len(p) {
		err = io.EOF
	}
	return n, err
}

// WriteAt drops what falls past the end of the stream, as sectionRepairTarget
// does: the repair writes whole slices, and the last one is padded.
func (t *rangesRepairTarget) WriteAt(p []byte, off int64) (int, error) {
	return t.each(p, off, t.target.WriteAt)
}

func (t *rangesRepairTarget) each(p []byte, off int64, op func([]byte, int64) (int, error)) (int, error) {
	if off >= t.size() {
		return 0, io.EOF
	}
	if rest := t.size() - off; int64(len(p)) > rest {
		p = p[:rest]
	}
	done := 0
	for _, r := range t.ranges {
		if done == len(p) {
			break
		}
		pos := off + int64(done)
		if pos >= r.virtual+r.length {
			continue
		}
		at := pos - r.virtual
		chunk := p[done:]
		if int64(len(chunk)) > r.length-at {
			chunk = chunk[:r.length-at]
		}
		n, err := op(chunk, r.file+at)
		done += n
		if err != nil && err != io.EOF {
			return done, err
		}
		if n < len(chunk) {
			return done, io.ErrUnexpectedEOF
		}
	}
	return done, nil
}

// RepairTarArchive извлекает .tarext/par2/recovery.par2 из TAR-архива (Stream 2)
// и чинит архив in-place (в том числе многотомный) через unxed/par2.
func RepairTarArchive(filename string) error {
	mvr, totalSize, err := tar.OpenMultiVolume(filename, os.O_RDWR)
	if err != nil {
		return err
	}
	defer mvr.Close()

	method, err := tar.DetectFormat(mvr)
	if err != nil {
		return err
	}

	shadowStart, shadowSize, err := tar.LocateShadowStream(mvr, totalSize, method)
	if err != nil || shadowSize == 0 {
		externalParPath := filename + ".par2"
		parData, err := os.ReadFile(externalParPath)
		if err == nil && len(parData) > 0 {
			srt := &sectionRepairTarget{target: mvr, size: totalSize}
			return par2.RepairTargetData(srt, parData)
		}
		return fmt.Errorf("embedded recovery record or external .par2 sidecar not found")
	}

	var parDataBuf bytes.Buffer
	err = tar.ExtractShadowFileToWriter(mvr, totalSize, method, ".tarext/par2/recovery.par2", &parDataBuf)
	if err != nil || parDataBuf.Len() == 0 {
		return fmt.Errorf("embedded recovery record not found or extraction failed: %v", err)
	}
	parData := parDataBuf.Bytes()

	srt := &sectionRepairTarget{target: mvr, size: shadowStart}
	return par2.RepairTargetData(srt, parData)
}

// GenerateExternalPar2 генерирует внешний файл .par2 рядом с архивом.
func GenerateExternalPar2(filename string, pct int) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	parData, err := par2.GeneratePAR2Stream(f, fi.Size(), filepath.Base(filename), pct)
	if err != nil {
		return err
	}

	return os.WriteFile(filename+".par2", parData, 0644)
}
