package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	base := strings.ToLower(filepath.Base(os.Args[0]))
	base = strings.TrimSuffix(base, ".exe")

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