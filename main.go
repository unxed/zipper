package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
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
		revision := ""
		buildTime := ""
		modified := ""
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					revision = setting.Value
				case "vcs.time":
					buildTime = setting.Value
				case "vcs.modified":
					if setting.Value == "true" {
						modified = " (dirty)"
					}
				}
			}
		}

		if revision != "" {
			shortRev := revision
			if len(shortRev) > 8 {
				shortRev = shortRev[:8]
			}
			fmt.Printf("%s version %s (%s%s, built %s)\n", base, Version, shortRev, modified, buildTime)
		} else {
			fmt.Printf("%s version %s\n", base, Version)
		}
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
		fmt.Println("Usage: tar [ctxrzjdvP] <archive> [files...]")
		fmt.Println("Options:")
		fmt.Println("  -c             Create archive")
		fmt.Println("  -x             Extract archive")
		fmt.Println("  -t             List archive contents")
		fmt.Println("  -r             Append files to archive")
		fmt.Println("  -d, --delete   Delete files from archive")
		fmt.Println("  -z             Gzip compression")
		fmt.Println("  -j             Bzip2 compression")
		fmt.Println("  -J             Xz compression")
		fmt.Println("  --zstd         Zstd compression")
		fmt.Println("  -f <archive>   Archive file path")
		fmt.Println("  -P <password>  Password for encryption/decryption")
	case "zip":
		fmt.Println("zip - compressor emulator")
		fmt.Println("Usage: zip [-r] [-0..-9] [-e] [-d] [-P password] <archive> [files...]")
		fmt.Println("Options:")
		fmt.Println("  -r             Recursive (always enabled)")
		fmt.Println("  -0..-9         Set compression level (0 = store)")
		fmt.Println("  -e             Encrypt Central Directory")
		fmt.Println("  -d             Delete files from archive")
		fmt.Println("  -P <password>  Set password for AES encryption")
	case "unzip":
		fmt.Println("unzip - extractor emulator")
		fmt.Println("Usage: unzip <archive> [-d outdir] [-o] [-n] [-P password]")
		fmt.Println("Options:")
		fmt.Println("  -d <outdir>    Output directory")
		fmt.Println("  -o             Overwrite files without prompting")
		fmt.Println("  -n             Never overwrite existing files")
		fmt.Println("  -l             List archive contents")
		fmt.Println("  -P <password>  Password for decryption")
	default:
		fmt.Printf("zipper %s - modern cross-platform archiver\n", Version)
		fmt.Println("Usage: zipper <command> [options] <archive> [files...]")
		fmt.Println("Commands:")
		fmt.Println("  c              Create archive")
		fmt.Println("  x              Extract archive")
		fmt.Println("  l              List archive contents")
		fmt.Println("  a              Append files to archive")
		fmt.Println("  d              Delete files from archive")
		fmt.Println("  repair         Repair archive using embedded recovery record")
		fmt.Println("\nGlobal Options:")
		fmt.Println("  -C <dir>       Change to directory <dir>")
		fmt.Println("  -j <threads>   Set concurrency threads limit")
		fmt.Println("  -l <level>     Compression level (1-9)")
		fmt.Println("  -m <method>    Compression method (deflate, zstd, store, gzip, bzip2, xz, lzma)")
		fmt.Println("  -solid         Use solid compression (zip)")
		fmt.Println("  -seek-chunk    Set seek index block size (for fast solid access)")
		fmt.Println("  -seek-continuous Use continuous seek index (GZIDX) instead of chunked (SOZip)")
		fmt.Println("  -p <password>  Set password for encryption/decryption")
		fmt.Println("  -e             Encrypt Central Directory (CDE) for ZIP")
		fmt.Println("  -incremental   Incremental backup/restore (.zip_dumpdir)")
		fmt.Println("  -progress      Show progress bar")
		fmt.Println("  -keep-old      Do not overwrite existing files during extraction")
		fmt.Println("  -keep-newer    Only overwrite if archive file is newer")
		fmt.Println("  -keep-broken   Keep extracted files even if CRC/decryption fails")
		fmt.Println("  -sparse        Write files as sparse blocks (seeking over zeros)")
		fmt.Println("  -tolerant      Tolerant mode (skip corrupted files and continue)")
		fmt.Println("  -index <path>  Path to SQLite index file (tar)")
		fmt.Println("  -embedded-index Embed index in TAR archive (F4SS)")
		fmt.Println("  -torrentzip    Create torrentzip compatible archive (zip)")
		fmt.Println("  -rr <pct>      Add recovery record (percentage, e.g. 5 for 5%)")
		fmt.Println("  -rr-external   Write recovery record to a separate .par2 file instead of embedding it")
		fmt.Println("  -lock          Lock archive to prevent further modifications")
		fmt.Println("  -v <size>      Volume size (e.g. 10M, 1G) for multi-volume archives")
		fmt.Println("  -no-platform-meta Do not include local platform metadata in ZIP")
		fmt.Println("  -no-times      Do not restore file modification times")
		fmt.Println("  -strip-components <num> Strip <num> leading components from file names")
		fmt.Println("  -max-file-size <bytes> Max allowed file size for extraction")
		fmt.Println("  -max-ratio <ratio> Max allowed decompression ratio")
		fmt.Println("  -exclude <pattern> Exclude files matching pattern (can be used multiple times)")
		fmt.Println("  -trim-parents  Trim parent directories from targets (like 7z)")
	}
}
