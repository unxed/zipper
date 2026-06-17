package archive

import (
    "path/filepath"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

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
	if off >= s.size { return 0, io.EOF }
	if off+int64(len(p)) > s.size { p = p[:s.size-off] }
	return s.target.ReadAt(p, off)
}

func (s *sectionRepairTarget) WriteAt(p []byte, off int64) (n int, err error) {
	if off >= s.size { return 0, fmt.Errorf("write out of bounds") }
	if off+int64(len(p)) > s.size { p = p[:s.size-off] }
	return s.target.WriteAt(p, off)
}

// RepairZipArchive извлекает скрытый по стандарту SOZip файл .recovery.par2
// и чинит архив in-place (в том числе многотомный) через unxed/par2.
func RepairZipArchive(filename string) error {
	mvr, totalSize, err := zip.OpenMultiVolume(filename, os.O_RDWR)
	if err != nil { return err }
	defer mvr.Close()

	var parOffset int64
	var parSize int64
	buf := make([]byte, 30)

	for off := int64(0); off < totalSize-30; {
		if _, err := mvr.ReadAt(buf[:4], off); err != nil { break }
		if binary.LittleEndian.Uint32(buf[:4]) == 0x04034b50 { // LFH Signature
			if _, err := mvr.ReadAt(buf[4:], off+4); err != nil { break }
			nlen := binary.LittleEndian.Uint16(buf[26:28])
			elen := binary.LittleEndian.Uint16(buf[28:30])
			nameBuf := make([]byte, nlen)
			if _, err := mvr.ReadAt(nameBuf, off+30); err != nil { break }
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
	dataStart := parOffset + 30 + int64(len(".recovery.par2"))
	sr := io.NewSectionReader(mvr, dataStart, parSize)
	if _, err := io.CopyBuffer(&parData, sr, make([]byte, 1024*1024)); err != nil {
		return fmt.Errorf("failed to read recovery payload: %w", err)
	}

	srt := &sectionRepairTarget{target: mvr, size: parOffset}
	return par2.RepairTargetData(srt, parData.Bytes())
}

// RepairTarArchive извлекает .tarext/par2/recovery.par2 из TAR-архива (Stream 2)
// и чинит архив in-place (в том числе многотомный) через unxed/par2.
func RepairTarArchive(filename string) error {
	mvr, totalSize, err := tar.OpenMultiVolume(filename, os.O_RDWR)
	if err != nil { return err }
	defer mvr.Close()

	method, err := tar.DetectFormat(mvr)
	if err != nil { return err }

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

	shadowBytes := make([]byte, shadowSize)
	if _, err := mvr.ReadAt(shadowBytes, shadowStart); err != nil {
		return err
	}

	var parData []byte
	sr := bytes.NewReader(shadowBytes)
	tr := tar.NewReader(sr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { return err }
		if hdr.Name == ".tarext/par2/recovery.par2" {
			parData = make([]byte, hdr.Size)
			io.ReadFull(tr, parData)
			break
		}
	}

	if len(parData) == 0 {
		return fmt.Errorf("embedded recovery record (.tarext/par2/recovery.par2) not found")
	}

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
