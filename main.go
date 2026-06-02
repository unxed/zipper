package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = "1.0.0"

func main() {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	base = strings.TrimSuffix(base, ".exe")

	// Показываем помощь, если утилита вызвана вообще без параметров
	if len(os.Args) < 2 {
		showHelp(base)
		return
	}

	// Перехват глобальных флагов версии и помощи
	arg := os.Args[1]
	if arg == "--version" || arg == "-v" || arg == "version" {
		fmt.Printf("%s version %s\n", base, Version)
		return
	}
	if arg == "--help" || arg == "-h" || arg == "help" {
		showHelp(base)
		return
	}

	var err error
	switch base {
	case "tar":
		err = runTar(os.Args)
	case "zip":
		err = runZip(os.Args)
	case "unzip":
		err = runUnzip(os.Args)
	default:
		err = runZipper(os.Args)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func showHelp(base string) {
	switch base {
	case "tar":
		fmt.Println("tar - tape archiver emulator")
		fmt.Println("Usage: tar [ctxzjJvf] <archive> [files...]")
		fmt.Println("Options:")
		fmt.Println("  -c             Create archive")
		fmt.Println("  -x             Extract archive")
		fmt.Println("  -z             Gzip compression")
		fmt.Println("  -j             Bzip2 compression")
		fmt.Println("  -J             Xz compression")
		fmt.Println("  --zstd         Zstd compression")
		fmt.Println("  -f <archive>   Archive file path")
	case "zip":
		fmt.Println("zip - compressor emulator")
		fmt.Println("Usage: zip [-r] [-0] [-e] [-P password] <archive> [files...]")
		fmt.Println("Options:")
		fmt.Println("  -r             Recursive (always enabled)")
		fmt.Println("  -0             Store only (no compression)")
		fmt.Println("  -e             Encrypt Central Directory")
		fmt.Println("  -P <password>  Set password for AES encryption")
	case "unzip":
		fmt.Println("unzip - extractor emulator")
		fmt.Println("Usage: unzip <archive> [-d outdir] [-o] [-n] [-P password]")
		fmt.Println("Options:")
		fmt.Println("  -d <outdir>    Output directory")
		fmt.Println("  -o             Overwrite files without prompting")
		fmt.Println("  -n             Never overwrite existing files")
		fmt.Println("  -P <password>  Password for decryption")
	default:
		fmt.Println("zipper - modern cross-platform archiver")
		fmt.Println("Usage: zipper <command> [options] <archive> [files...]")
		fmt.Println("Commands:")
		fmt.Println("  c              Create archive")
		fmt.Println("  x              Extract archive")
		fmt.Println("\nGlobal Options:")
		fmt.Println("  -C <dir>       Change to directory <dir>")
		fmt.Println("  -j <threads>   Set concurrency threads limit")
		fmt.Println("  -m <method>    Compression method (deflate, zstd, store, gzip, bzip2, xz, lzma)")
		fmt.Println("  -solid         Use solid compression (ZIP solid-in-solid)")
		fmt.Println("  -seek-chunk    Set seek index block size (for fast solid access)")
		fmt.Println("  -seek-continuous Use continuous seek index (GZIDX) instead of chunked (SOZip)")
		fmt.Println("  -p <password>  Set password for encryption/decryption")
		fmt.Println("  -e             Encrypt Central Directory (CDE) for ZIP")
		fmt.Println("  -incremental   Incremental backup/restore (.zip_dumpdir)")
		fmt.Println("  -keep-old      Do not overwrite existing files during extraction")
		fmt.Println("  -keep-newer    Only overwrite if archive file is newer")
		fmt.Println("  -keep-broken   Keep extracted files even if CRC/decryption fails")
		fmt.Println("  -sparse        Write files as sparse blocks (seeking over zeros)")
		fmt.Println("  -tolerant      Tolerant mode (skip corrupted files and continue)")
		fmt.Println("  -index <path>  Path to SQLite index file (tar)")
		fmt.Println("  -embedded-index Embed index in TAR archive (F4SS)")
	}
}