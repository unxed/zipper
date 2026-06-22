package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// checkOverwrite checks if the archive file already exists and prompts the user for confirmation.
func checkOverwrite(archivePath string) error {
	if archivePath == "-" {
		return nil
	}
	if _, err := os.Stat(archivePath); err == nil {
		fmt.Fprintf(os.Stderr, "File '%s' already exists. Overwrite? [y/N]: ", archivePath)
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("operation cancelled")
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("operation cancelled")
		}
	}
	return nil
}
