package main

import (
    "strings"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unxed/zipper/archive"
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(value string) error { *s = append(*s, value); return nil }
func runZipper(args []string) error {
	// Предварительный поиск команды среди аргументов (чтобы флаги могли стоять ДО команды)
	var cmd string
	var cmdIdx int = -1
	for i := 1; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			cmd = args[i]
			cmdIdx = i
			break
		}
	}

	if cmd == "" || cmdIdx == -1 {
		return fmt.Errorf("usage: zipper [options] <command> <archive> [files...]\nCommands: c (create), x (extract), l (list), a (append), d (delete), repair")
	}

	fs := flag.NewFlagSet("zipper", flag.ContinueOnError)

	var (
		outDir         string
		concurrency    int
		level          int
		xattrs         bool
		splitSizeStr   string
		solid          bool
		method         string
		incremental    bool
		keepOld        bool
		keepNewer      bool
		keepBroken     bool
		sparse         bool
		tolerant       bool
		password       string
		encryptCD      bool
		seekChunkSize  int
		seekContinuous bool
		indexPath      string
		embeddedIndex  bool
		torrentZip     bool
		recoveryPct    int
		noPlatformMeta bool
		noTimes        bool
		stripComp      int
		maxFileSize    int64
		maxRatio       int64
		recoveryExternal bool
		lock           bool
		excludes       stringSlice
		progress       bool
		trimParents    bool
	)

	fs.Var(&excludes, "exclude", "Exclude files matching pattern")
	fs.BoolVar(&progress, "progress", false, "Show progress bar")
	fs.BoolVar(&trimParents, "trim-parents", false, "Trim parent directories from targets (like 7z)")
	fs.StringVar(&outDir, "C", ".", "Change to directory")
	fs.IntVar(&concurrency, "j", 0, "Concurrency")
	fs.IntVar(&level, "l", 0, "Compression level (1-9)")
	fs.StringVar(&password, "p", "", "Password for encryption/decryption")
	fs.BoolVar(&encryptCD, "e", false, "Encrypt Central Directory (CDE)")
	fs.IntVar(&seekChunkSize, "seek-chunk", 0, "Seek chunk size for solid archives (e.g. 1048576)")
	fs.BoolVar(&seekContinuous, "seek-continuous", false, "Use continuous seek index (GZIDX) instead of chunked (SOZip)")
	fs.BoolVar(&xattrs, "xattrs", true, "Preserve xattrs")
	fs.BoolVar(&solid, "solid", false, "Use solid compression (zip)")
	fs.StringVar(&method, "m", "", "Compression method (deflate, zstd, store, etc.)")
	fs.BoolVar(&incremental, "incremental", false, "Incremental mode (.zip_dumpdir)")
	fs.BoolVar(&keepOld, "keep-old", false, "Keep old files on extract")
	fs.BoolVar(&keepNewer, "keep-newer", false, "Keep newer files on extract")
	fs.BoolVar(&keepBroken, "keep-broken", false, "Keep broken files")
	fs.BoolVar(&sparse, "sparse", false, "Sparse extraction")
	fs.BoolVar(&tolerant, "tolerant", false, "Tolerant extraction (ignore some corruptions)")
	fs.StringVar(&indexPath, "index", "", "Path to SQLite index file")
	fs.BoolVar(&embeddedIndex, "embedded-index", true, "Embed index in TAR archive (F4SS)")
	fs.BoolVar(&torrentZip, "torrentzip", false, "Create torrentzip compatible archive (zip)")
	fs.IntVar(&recoveryPct, "rr", 0, "Add recovery record (percentage, e.g. 5 for 5%)")
	fs.BoolVar(&recoveryExternal, "rr-external", false, "Write recovery record to a separate .par2 file instead of embedding it")
	fs.BoolVar(&lock, "lock", false, "Lock archive to prevent further modifications")
	fs.StringVar(&splitSizeStr, "v", "", "Volume size (e.g. 100M, 1G) for multi-volume archives")
	fs.BoolVar(&noPlatformMeta, "no-platform-meta", false, "Do not include local platform metadata in ZIP")
	fs.BoolVar(&noTimes, "no-times", false, "Do not restore file modification times")
	fs.IntVar(&stripComp, "strip-components", 0, "Strip number of leading components from file names")
	fs.Int64Var(&maxFileSize, "max-file-size", 0, "Max allowed file size for extraction")
	fs.Int64Var(&maxRatio, "max-ratio", 0, "Max allowed decompression ratio")

	// Собираем все аргументы, кроме самой команды, для парсинга флагов
	flagArgs := append([]string{}, args[1:cmdIdx]...)
	flagArgs = append(flagArgs, args[cmdIdx+1:]...)

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	parsedArgs := fs.Args()

	if len(parsedArgs) < 1 {
		return fmt.Errorf("archive name is required")
	}

	archivePath := parsedArgs[0]
	if archivePath != "-" && filepath.Ext(archivePath) == "" {
		archivePath += archive.DefaultFormat()
	}

	splitSize, err := parseSize(splitSizeStr)
	if err != nil {
		return fmt.Errorf("invalid volume size: %v", err)
	}

	opts := archive.Options{
		SplitSize:      splitSize,
		Concurrency:    concurrency,
		Xattrs:         xattrs,
		Solid:          solid,
		Method:         method,
		Level:          level,
		Incremental:    incremental,
		KeepOldFiles:   keepOld,
		KeepNewerFiles: keepNewer,
		KeepBroken:    keepBroken,
		Sparse:        sparse,
		Tolerant:      tolerant,
		Password:       password,
		EncryptCD:      encryptCD,
		SeekChunkSize:  uint32(seekChunkSize),
		SeekContinuous: seekContinuous,
		IndexPath:      indexPath,
		EmbeddedIdx:    embeddedIndex,
		TorrentZip:     torrentZip,
		RecoveryPct:    recoveryPct,
		NoPlatformMetadata: noPlatformMeta,
		NoTimes:            noTimes,
		StripComponents:    stripComp,
		MaxFileSize:        maxFileSize,
		MaxRatio:           maxRatio,
		RecoveryExternal:   recoveryExternal,
		Lock:               lock,
	}

	switch cmd {
	case "l":
		return runList(archivePath, opts)

	case "c":
		if len(parsedArgs) < 2 {
			return fmt.Errorf("please specify files to archive")
		}

		absChroot, err := filepath.Abs(outDir)
		if err != nil {
			return fmt.Errorf("invalid chroot directory: %w", err)
		}

		files := make(map[string]os.FileInfo)
		pathMapping := make(map[string]string)
		var totalBytes, totalEntries int64
		for _, target := range parsedArgs[1:] {
			targetPath := target
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(absChroot, targetPath)
			}
			targetPath = filepath.Clean(targetPath)
			baseDir := filepath.Dir(targetPath)

			err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				for _, ex := range excludes {
					if matched, _ := filepath.Match(ex, info.Name()); matched {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}
				if path != absChroot {
					files[path] = info
					if trimParents {
						rel, relErr := filepath.Rel(baseDir, path)
						if relErr == nil {
							pathMapping[path] = filepath.ToSlash(rel)
						}
					}
					totalBytes += info.Size()
					totalEntries++
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk %s: %w", target, err)
			}
		}
		if trimParents {
			opts.PathMapping = pathMapping
		}

		a, err := archive.NewArchiver(archivePath, absChroot, opts)
		if err != nil {
			return fmt.Errorf("failed to create archiver: %w", err)
		}
		defer a.Close()

		var stopProgress func()
		if progress {
			stopProgress = startProgressBar(a, totalBytes, totalEntries, "Archiving")
		}
		err = a.Archive(context.Background(), files)
		if stopProgress != nil {
			stopProgress()
		}
		if err != nil {
			return err
		}
		return nil

	case "a":
		if len(parsedArgs) < 2 {
			return fmt.Errorf("please specify files to append")
		}
		absChroot, err := filepath.Abs(outDir)
		if err != nil {
			return fmt.Errorf("invalid chroot directory: %w", err)
		}
		u, err := archive.NewUpdater(archivePath, opts)
		if err != nil {
			return fmt.Errorf("failed to initialize updater: %w", err)
		}
		defer u.Close()

		for _, target := range parsedArgs[1:] {
			targetPath := target
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(absChroot, targetPath)
			}
			targetPath = filepath.Clean(targetPath)
			baseDir := filepath.Dir(targetPath)

			fi, err := os.Stat(targetPath)
			if err != nil {
				return fmt.Errorf("failed to read file info for %s: %w", targetPath, err)
			}
			f, err := os.Open(targetPath)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", targetPath, err)
			}

			nameInArchive := filepath.ToSlash(target)
			if trimParents {
				rel, relErr := filepath.Rel(baseDir, targetPath)
				if relErr == nil {
					nameInArchive = filepath.ToSlash(rel)
				}
			}
			err = u.Append(nameInArchive, fi.Size(), f)
			f.Close()
			if err != nil {
				return fmt.Errorf("failed to append %s: %w", target, err)
			}
		}
		return nil

	case "d":
		if len(parsedArgs) < 2 {
			return fmt.Errorf("please specify files to delete")
		}
		u, err := archive.NewUpdater(archivePath, opts)
		if err != nil {
			return fmt.Errorf("failed to initialize updater: %w", err)
		}
		defer u.Close()

		for _, target := range parsedArgs[1:] {
			err = u.Remove(filepath.ToSlash(target))
			if err != nil {
				return fmt.Errorf("failed to delete %s: %w", target, err)
			}
		}
		return nil

	case "x":
		absOut, err := filepath.Abs(outDir)
		if err != nil {
			return fmt.Errorf("invalid output directory: %w", err)
		}
		if err := os.MkdirAll(absOut, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		e, err := archive.NewExtractor(archivePath, absOut, opts)
		if err != nil {
			return fmt.Errorf("failed to create extractor: %w", err)
		}
		defer e.Close()

		var stopProgress func()
		if progress {
			stopProgress = startProgressBar(e, 0, 0, "Extracting")
		}
		err = e.Extract(context.Background())
		if stopProgress != nil {
			stopProgress()
		}
		if err != nil {
			return err
		}
		return nil
	case "repair":
		if len(parsedArgs) < 1 {
			return fmt.Errorf("archive name is required for repair")
		}
		repairPath := parsedArgs[0]
		fmt.Printf("Attempting to repair %s...\n", repairPath)
		
		fmtType := archive.DetectFormat(repairPath)
		if fmtType == "zip" {
			// Вызов метода ремонта из нашего адаптера, использующего unxed/par2
			if err := archive.RepairZipArchive(repairPath); err != nil {
				return fmt.Errorf("repair error: %w", err)
			}
		} else if fmtType == "tar" {
			// Вызов метода ремонта из нашего адаптера, использующего unxed/par2
			if err := archive.RepairTarArchive(repairPath); err != nil {
				return fmt.Errorf("repair error: %w", err)
			}
		} else {
			return fmt.Errorf("unsupported archive format for recovery")
		}
		fmt.Println("Repair successful!")
		return nil

	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func parseSize(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.ToUpper(s)
	var multiplier int64 = 1
	switch s[len(s)-1] {
	case 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'G':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	var val int64
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return 0, err
	}
	return val * multiplier, nil
}