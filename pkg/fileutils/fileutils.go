package fileutils

import (
	"fmt"
	"os"
)

// EnsureDirectoryExists creates the specified directory if it does not exist
func EnsureDirectoryExists(path string) error {
	if path == "" {
		return fmt.Errorf("empty path provided")
	}
	return os.MkdirAll(path, os.ModePerm)
}

// FileExists checks if a file exists at the specified path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
