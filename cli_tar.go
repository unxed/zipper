package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/unxed/zipper/archive"
)

// runTar эмулирует поведение традиционной утилиты tar
func runTar(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("tar: missing arguments")
	}

	mode := ""
	archivePath := ""
	opts := archive.Options{Xattrs: true} // По умолчанию сохраняем расширенные атрибуты
	var files []string

	var excludes []string

	var progress bool

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--zstd" || arg == "-a" {
			opts.Method = "zstd"
			continue
		}
		if arg == "--delete" {
			mode = "d"
			continue
		}
		if arg == "--append" || arg == "--update" {
			mode = "r"
			continue
		}
		if strings.HasPrefix(arg, "--exclude=") {
			excludes = append(excludes, strings.TrimPrefix(arg, "--exclude="))
			continue
		}
		if arg == "--progress" {
			progress = true
			continue
		}
		if len(arg) == 2 && arg[0] == '-' && arg[1] >= '0' && arg[1] <= '9' {
			opts.Level = int(arg[1] - '0')
			if opts.Level == 0 {
				opts.Method = "store"
			}
			continue
		}
		if !strings.HasPrefix(arg, "-") && mode == "" {
			arg = "-" + arg
		}
		if strings.HasPrefix(arg, "-") {
			for j := 1; j < len(arg); j++ {
				ch := arg[j]
				switch ch {
				case 'c':
					mode = "c"
				case 'x':
					mode = "x"
				case 't':
					mode = "t"
				case 'r':
					mode = "r"
				case 'd':
					mode = "d"
				case 'z':
					opts.Method = "gzip"
				case 'j':
					opts.Method = "bzip2"
				case 'J':
					opts.Method = "xz"
				case 'v': // Игнорируем verbose
				case 'f':
					if j+1 < len(arg) {
						archivePath = arg[j+1:]
					} else if i+1 < len(args) {
						archivePath = args[i+1]
						i++
					} else {
						return fmt.Errorf("tar: option requires an argument -- f")
					}
					goto nextArg
				case 'P':
					if j+1 < len(arg) {
						opts.Password = arg[j+1:]
					} else if i+1 < len(args) {
						opts.Password = args[i+1]
						i++
					} else {
						return fmt.Errorf("tar: option requires an argument -- P")
					}
					goto nextArg
				}
			}
		} else {
			files = append(files, arg)
		}
	nextArg:
	}

	if archivePath == "" {
		archivePath = "-"
	}

	if mode == "t" {
		return runList(archivePath, opts)
	} else if mode == "c" {
		if len(files) == 0 {
			return fmt.Errorf("tar: cowardly refusing to create an empty archive")
		}
		a, err := archive.NewArchiver(archivePath, ".", opts)
		if err != nil {
			return err
		}

		fMap := make(map[string]os.FileInfo)
		var totalBytes, totalEntries int64
		for _, f := range files {
			err := filepath.WalkDir(f, func(path string, d fs.DirEntry, err error) error {
				if err == nil && path != "." {
					for _, ex := range excludes {
						if matched, _ := filepath.Match(ex, d.Name()); matched {
							if d.IsDir() {
								return filepath.SkipDir
							}
							return nil
						}
					}
					info, err := d.Info()
					if err != nil {
						return err
					}
					fMap[path] = info
					totalBytes += info.Size()
					totalEntries++
				}
				return err
			})
			if err != nil {
				a.Close()
				return err
			}
		}
		var stopProgress func()
		if progress {
			stopProgress = startProgressBar(a, totalBytes, totalEntries, "Archiving")
		}
		archiveErr := a.Archive(context.Background(), fMap)
		if stopProgress != nil {
			stopProgress()
		}

		closeErr := a.Close()
		if archiveErr != nil {
			return archiveErr
		}
		return closeErr
	} else if mode == "x" {
		e, err := archive.NewExtractor(archivePath, ".", opts)
		if err != nil {
			return err
		}

		var stopProgress func()
		if progress {
			stopProgress = startProgressBar(e, 0, 0, "Extracting")
		}
		extractErr := e.Extract(context.Background())
		if stopProgress != nil {
			stopProgress()
		}

		closeErr := e.Close()
		if extractErr != nil {
			return extractErr
		}
		return closeErr
	} else if mode == "r" {
		u, err := archive.NewUpdater(archivePath, opts)
		if err != nil {
			return err
		}
		defer u.Close()

		for _, f := range files {
			fi, err := os.Stat(f)
			if err != nil {
				return err
			}
			file, err := os.Open(f)
			if err != nil {
				return err
			}
			err = u.Append(f, fi.Size(), file)
			file.Close()
			if err != nil {
				return err
			}
		}
		return nil
	} else if mode == "d" {
		u, err := archive.NewUpdater(archivePath, opts)
		if err != nil {
			return err
		}
		defer u.Close()

		for _, f := range files {
			if err := u.Remove(f); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("tar: must specify action (-c, -x, -r, or --delete)")
}
