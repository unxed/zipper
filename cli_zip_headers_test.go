package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCli_ZipEntriesPointAtTheirOwnLocalHeaders is the regression test for
// issue #16: 7-Zip, and ArcLite in Far Manager through it, reported "Headers
// Error" for every directory of an archive made from a directory tree, while
// an archive of a single file tested clean.
//
// The central directory record of an entry carries the offset of that entry's
// local file header (APPNOTE 4.4.16). Directory entries were recorded in the
// central directory only, with no local header written, so their offset landed
// on the local header of the file that followed. When testing, 7-Zip reads the
// local header at that offset and compares it with the central record
// (CInArchive::Read_LocalItem_After_CdItem and AreItemsEqual in ZipIn.cpp); a
// different compression method is a header error. The test checks the same
// thing: at every recorded offset there is a local header of the same entry.
func TestCli_ZipEntriesPointAtTheirOwnLocalHeaders(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub", "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	// Compressible content, so the file after each directory is deflated and a
	// directory offset landing on it cannot pass for a stored entry.
	payload := []byte(strings.Repeat("issue 16 ", 512))
	for _, name := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
		if err := os.WriteFile(filepath.Join(srcDir, name), payload, 0644); err != nil {
			t.Fatal(err)
		}
	}

	archivePath := filepath.Join(tmp, "issue16.zip")
	if err := runZipper([]string{"zipper", "c", "-C", tmp, archivePath, "src"}); err != nil {
		t.Fatalf("runZipper(c) failed: %v", err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	eocd := bytes.LastIndex(data, []byte("PK\x05\x06"))
	if eocd < 0 || eocd+22 > len(data) {
		t.Fatalf("end of central directory record not found")
	}
	le := binary.LittleEndian
	total := int(le.Uint16(data[eocd+10:]))
	cdOffset := le.Uint32(data[eocd+16:])
	if cdOffset == 0xFFFFFFFF || total == 0xFFFF {
		t.Fatalf("unexpected zip64 end of central directory in a small archive")
	}

	var dirs, files int
	p := int(cdOffset)
	for i := 0; i < total; i++ {
		if p+46 > len(data) || le.Uint32(data[p:]) != 0x02014b50 {
			t.Fatalf("central directory record %d not found at offset %d", i, p)
		}
		cdFlags := le.Uint16(data[p+8:])
		cdMethod := le.Uint16(data[p+10:])
		nameLen := int(le.Uint16(data[p+28:]))
		extraLen := int(le.Uint16(data[p+30:]))
		commentLen := int(le.Uint16(data[p+32:]))
		localOffset := int(le.Uint32(data[p+42:]))
		name := string(data[p+46 : p+46+nameLen])
		p += 46 + nameLen + extraLen + commentLen

		if strings.HasSuffix(name, "/") {
			dirs++
		} else {
			files++
		}

		if localOffset+30 > len(data) || le.Uint32(data[localOffset:]) != 0x04034b50 {
			t.Errorf("%q: no local file header at recorded offset %d", name, localOffset)
			continue
		}
		localFlags := le.Uint16(data[localOffset+6:])
		localMethod := le.Uint16(data[localOffset+8:])
		localNameLen := int(le.Uint16(data[localOffset+26:]))
		localName := string(data[localOffset+30 : localOffset+30+localNameLen])

		if localName != name {
			t.Errorf("%q: recorded offset %d holds the local header of %q", name, localOffset, localName)
			continue
		}
		if localMethod != cdMethod {
			t.Errorf("%q: method %d in local header, %d in central directory", name, localMethod, cdMethod)
		}
		// Bit 3 (data descriptor) may legitimately differ; 7-Zip ignores it too.
		if (localFlags^cdFlags)&^0x0008 != 0 {
			t.Errorf("%q: flags %#04x in local header, %#04x in central directory", name, localFlags, cdFlags)
		}
	}

	if dirs < 3 || files < 2 {
		t.Fatalf("archive holds %d directories and %d files, want at least 3 and 2", dirs, files)
	}
}
