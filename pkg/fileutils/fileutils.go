package fileutils

import (
	"fmt"
	"os"
)

func EnsureDirectoryExists(path string) error {
	if path == "" {
		return fmt.Errorf("empty path provided")
	}
	return os.MkdirAll(path, os.ModePerm)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
