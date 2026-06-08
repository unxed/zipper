package archive

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/unxed/par2"
	"github.com/unxed/tar"
)

// RepairZipArchive извлекает скрытый по стандарту SOZip файл .recovery.par2,
// временно отрезает его, чинит ZIP нативным методом par2.RepairFile и сшивает обратно.
func RepairZipArchive(filename string) error {
	f, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	size := stat.Size()

	var parOffset int64
	var parSize int64
	buf := make([]byte, 30)

	for off := int64(0); off < size-30; {
		if _, err := f.ReadAt(buf[:4], off); err != nil {
			break
		}
		if binary.LittleEndian.Uint32(buf[:4]) == 0x04034b50 { // LFH Signature
			if _, err := f.ReadAt(buf[4:], off+4); err != nil {
				break
			}
			nlen := binary.LittleEndian.Uint16(buf[26:28])
			elen := binary.LittleEndian.Uint16(buf[28:30])
			nameBuf := make([]byte, nlen)
			if _, err := f.ReadAt(nameBuf, off+30); err != nil {
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
		return fmt.Errorf("embedded recovery record (.recovery.par2) not found")
	}

	// Читаем полезную нагрузку
	parData := make([]byte, parSize)
	dataStart := parOffset + 30 + int64(len(".recovery.par2"))
	if _, err := f.ReadAt(parData, dataStart); err != nil {
		return fmt.Errorf("failed to read recovery payload: %w", err)
	}

	// Отрезаем избыточные данные для восстановления оригинального вида перед ремонтом
	if err := f.Truncate(parOffset); err != nil {
		return err
	}

	cleanData := make([]byte, parOffset)
	if _, err := f.ReadAt(cleanData, 0); err != nil {
		return err
	}
	f.Close()

	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, cleanData, 0644); err != nil {
		return err
	}
	defer os.Remove(tempFile)

	// Запускаем ремонт из нашей нативной библиотеки unxed/par2
	if err := par2.RepairFile(tempFile, parData); err != nil {
		return err
	}

	repairedData, err := os.ReadFile(tempFile)
	if err != nil {
		return err
	}

	// Восстанавливаем оригинальную структуру со скрытым файлом
	f, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	f.Write(repairedData)

	lfhBuf := make([]byte, 30+len(".recovery.par2"))
	binary.LittleEndian.PutUint32(lfhBuf[0:4], 0x04034b50)
	binary.LittleEndian.PutUint16(lfhBuf[26:28], uint16(len(".recovery.par2")))
	copy(lfhBuf[30:], ".recovery.par2")
	f.Write(lfhBuf)
	f.Write(parData)

	return nil
}

// RepairTarArchive извлекает .tarext/par2/recovery.par2 из TAR-архива (Stream 2),
// временно отрезает его, чинит TAR нативным методом par2.RepairFile и сшивает обратно.
func RepairTarArchive(filename string) error {
	f, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	size := stat.Size()

	method, err := tar.DetectFormat(f)
	if err != nil {
		return err
	}

	shadowStart, shadowSize, err := tar.LocateShadowStream(f, size, method)
	if err != nil || shadowSize == 0 {
		return fmt.Errorf("no embedded F4SS metadata stream found: %w", err)
	}

	shadowBytes := make([]byte, shadowSize)
	if _, err := f.ReadAt(shadowBytes, shadowStart); err != nil {
		return err
	}

	// Вытаскиваем par2 из TAR-потока Stream 2
	var parData []byte
	sr := bytes.NewReader(shadowBytes)
	tr := tar.NewReader(sr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == ".tarext/par2/recovery.par2" {
			parData = make([]byte, hdr.Size)
			io.ReadFull(tr, parData)
			break
		}
	}

	if len(parData) == 0 {
		return fmt.Errorf("embedded recovery record (.tarext/par2/recovery.par2) not found")
	}

	if err := f.Truncate(shadowStart); err != nil {
		return err
	}

	cleanData := make([]byte, shadowStart)
	if _, err := f.ReadAt(cleanData, 0); err != nil {
		return err
	}
	f.Close()

	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, cleanData, 0644); err != nil {
		return err
	}
	defer os.Remove(tempFile)

	// Запускаем ремонт из нашей нативной библиотеки unxed/par2
	if err := par2.RepairFile(tempFile, parData); err != nil {
		return err
	}

	repairedData, err := os.ReadFile(tempFile)
	if err != nil {
		return err
	}

	f, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	f.Write(repairedData)
	f.Write(shadowBytes)

	// Восстанавливаем Magic Footer
	tar.WriteMagicFooter(f, method, shadowStart, shadowSize)

	return nil
}