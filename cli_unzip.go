package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unxed/zipper/archive"
)

// runUnzip эмулирует поведение традиционной утилиты unzip
func runUnzip(args []string) error {
	opts := archive.Options{Xattrs: true}
	var archivePath string
	outDir := "."

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-d" {
			if i+1 < len(args) {
				outDir = args[i+1]
				i++
			}
		} else if arg == "-o" {
			opts.KeepOldFiles = false
			opts.KeepNewerFiles = false
		} else if arg == "-n" {
			opts.KeepOldFiles = true
		} else if arg == "-P" {
			if i+1 < len(args) {
				opts.Password = args[i+1]
				i++
			}
		} else if !strings.HasPrefix(arg, "-") && archivePath == "" {
			archivePath = arg
		}
	}

	if archivePath == "" {
		return fmt.Errorf("unzip: missing archive name")
	}
	if filepath.Ext(archivePath) == "" {
		archivePath += ".zip"
	}

	os.MkdirAll(outDir, 0755)
	e, err := archive.NewExtractor(archivePath, outDir, opts)
	if err != nil {
		return err
	}
	defer e.Close()

	return e.Extract(context.Background())
}