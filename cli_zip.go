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

// runZip эмулирует поведение традиционной утилиты zip
func runZip(args []string) error {
	opts := archive.Options{Xattrs: true}
	var archivePath string
	var files []string
	deleteMode := false

	var excludes []string

	var progress bool

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if arg == "-0" {
				opts.Method = "store"
				opts.Level = 0
			} else if len(arg) == 2 && arg[1] >= '1' && arg[1] <= '9' {
				opts.Level = int(arg[1] - '0')
			} else if arg == "-e" {
				opts.EncryptCD = true // Утилита zip обычно просит пароль интерактивно, для теста ставим флаг
			} else if arg == "-d" {
				deleteMode = true
			} else if arg == "-P" {
				if i+1 < len(args) {
					opts.Password = args[i+1]
					i++
				}
			} else if arg == "-x" {
				if i+1 < len(args) {
					excludes = append(excludes, args[i+1])
					i++
				}
			} else if arg == "--progress" || arg == "-progress" {
				progress = true
			}
			// Все остальные флаги (вроде -r) пока просто игнорируем
		} else {
			if archivePath == "" {
				archivePath = arg
			} else {
				files = append(files, arg)
			}
		}
	}

	if archivePath == "" {
		return fmt.Errorf("zip: missing archive name")
	}
	if archivePath != "-" && filepath.Ext(archivePath) == "" {
		archivePath += ".zip"
	}

	if deleteMode {
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

	a, err := archive.NewArchiver(archivePath, ".", opts)
	if err != nil {
		return err
	}
	defer a.Close()

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
			return err
		}
	}
	var stopProgress func()
	if progress {
		stopProgress = startProgressBar(a, totalBytes, totalEntries, "Archiving")
	}
	err = a.Archive(context.Background(), fMap)
	if stopProgress != nil {
		stopProgress()
	}
	return err
}