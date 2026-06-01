package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unxed/zipper/engine"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: zipper <command> [options] <archive> [files...]\nCommands: c (create), x (extract)")
	}

	cmd := args[1]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)

	var (
		outDir      string
		concurrency int
		xattrs      bool
		solid       bool
	)

	fs.StringVar(&outDir, "C", ".", "Change to directory")
	fs.IntVar(&concurrency, "j", 0, "Concurrency")
	fs.BoolVar(&xattrs, "xattrs", true, "Preserve xattrs")
	fs.BoolVar(&solid, "solid", false, "Use solid compression (zip)")

	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	parsedArgs := fs.Args()

	if len(parsedArgs) < 1 {
		return fmt.Errorf("archive name is required")
	}

	archivePath := parsedArgs[0]
	if filepath.Ext(archivePath) == "" {
		archivePath += engine.DefaultFormat()
	}

	opts := engine.Options{
		Concurrency: concurrency,
		Xattrs:      xattrs,
		Solid:       solid,
	}

	switch cmd {
	case "c":
		if len(parsedArgs) < 2 {
			return fmt.Errorf("please specify files to archive")
		}

		absChroot, err := filepath.Abs(outDir)
		if err != nil {
			return fmt.Errorf("invalid chroot directory: %w", err)
		}

		a, err := engine.NewArchiver(archivePath, absChroot, opts)
		if err != nil {
			return fmt.Errorf("failed to create archiver: %w", err)
		}
		defer a.Close()

		files := make(map[string]os.FileInfo)
		for _, target := range parsedArgs[1:] {
			targetPath := target
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(absChroot, targetPath)
			}
			err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if path != absChroot {
					files[path] = info
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk %s: %w", target, err)
			}
		}

		if err := a.Archive(context.Background(), files); err != nil {
			return fmt.Errorf("archive error: %w", err)
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

		e, err := engine.NewExtractor(archivePath, absOut, opts)
		if err != nil {
			return fmt.Errorf("failed to create extractor: %w", err)
		}
		defer e.Close()

		if err := e.Extract(context.Background()); err != nil {
			return fmt.Errorf("extract error: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}