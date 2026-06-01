package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unxed/zipper/engine"
)

// runZip эмулирует поведение традиционной утилиты zip
func runZip(args []string) error {
	opts := engine.Options{Xattrs: true}
	var archivePath string
	var files []string

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if arg == "-0" {
				opts.Method = "store"
			} else if arg == "-e" {
				opts.EncryptCD = true // Утилита zip обычно просит пароль интерактивно, для теста ставим флаг
			} else if arg == "-P" {
				if i+1 < len(args) {
					opts.Password = args[i+1]
					i++
				}
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
	if filepath.Ext(archivePath) == "" {
		archivePath += ".zip"
	}

	a, err := engine.NewArchiver(archivePath, ".", opts)
	if err != nil {
		return err
	}
	defer a.Close()

	fMap := make(map[string]os.FileInfo)
	for _, f := range files {
		err := filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
			if err == nil && path != "." {
				fMap[path] = info
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	return a.Archive(context.Background(), fMap)
}