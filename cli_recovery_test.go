package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestCli_EmbeddedRecoveryRecord: an archive written with -rr is repaired
// after damage to an entry's data and, separately, to its central directory,
// which the recovery data covers now that it sits in front of the directory.
func TestCli_EmbeddedRecoveryRecord(t *testing.T) {
	for _, where := range []string{"entry data", "central directory"} {
		t.Run(where, func(t *testing.T) {
			tmp := t.TempDir()
			body := bytes.Repeat([]byte("recovery record test line\n"), 20000)
			if err := os.WriteFile(filepath.Join(tmp, "data.txt"), body, 0o644); err != nil {
				t.Fatal(err)
			}
			arc := filepath.Join(tmp, "rr.zip")
			if err := runZipper([]string{"zipper", "c", "-rr", "10", "-C", tmp, arc, "data.txt"}); err != nil {
				t.Fatalf("c: %v", err)
			}
			raw, err := os.ReadFile(arc)
			if err != nil {
				t.Fatal(err)
			}
			eocd := bytes.LastIndex(raw, []byte("PK\x05\x06"))
			if eocd < 0 {
				t.Fatal("no end of central directory record")
			}
			at := 100
			if where == "central directory" {
				at = int(binary.LittleEndian.Uint32(raw[eocd+16:eocd+20])) + 20
			}
			raw[at] ^= 0xff
			if err := os.WriteFile(arc, raw, 0o644); err != nil {
				t.Fatal(err)
			}

			if err := runZipper([]string{"zipper", "repair", arc}); err != nil {
				t.Fatalf("repair: %v", err)
			}
			dst := filepath.Join(tmp, "dst")
			if err := runZipper([]string{"zipper", "x", "-C", dst, arc}); err != nil {
				t.Fatalf("x: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(dst, "data.txt"))
			if err != nil || !bytes.Equal(got, body) {
				t.Fatalf("data.txt did not come back as written: %v", err)
			}
		})
	}
}
