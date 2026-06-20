package main

import (
	"fmt"
	"io/fs"

	"github.com/unxed/zipper/archive"
)

func runList(archivePath string, opts archive.Options) error {
	fsys, err := archive.OpenFS(archivePath, opts)
	if err != nil {
		return err
	}
	defer fsys.Close()

	fmt.Printf("%10s  %16s  %s\n", "Length", "Date   Time", "Name")
	fmt.Printf("%10s  %16s  %s\n", "---------", "----------------", "----")

	var totalSize int64
	var count int

	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		if !d.IsDir() {
			totalSize += info.Size()
			fmt.Printf("%10d  %s  %s\n", info.Size(), info.ModTime().Format("2006-01-02 15:04"), path)
		} else {
			fmt.Printf("%10s  %s  %s\n", "-", info.ModTime().Format("2006-01-02 15:04"), path+"/")
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("%10s  %16s  %s\n", "---------", "----------------", "-------")
	fmt.Printf("%10d  %16s  %d files\n", totalSize, "", count)

	return nil
}
