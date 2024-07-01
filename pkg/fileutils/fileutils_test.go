package fileutils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureDirectoryExists(t *testing.T) {
	testCases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid path", "testdir/subdir", false},
		{"Empty path", "", true},
		{"Root path", "/", false}, // This might fail if the test is not run as root
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := EnsureDirectoryExists(tc.path)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.path != "/" {
					assert.DirExists(t, tc.path)
				}
			}
		})
	}

	// Cleanup
	os.RemoveAll("testdir")
}

func TestFileExists(t *testing.T) {
	testFile := "testfile.txt"

	// Create a test file
	f, err := os.Create(testFile)
	assert.NoError(t, err)
	f.Close()

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Existing file", testFile, true},
		{"Non-existent file", "nonexistent.txt", false},
		{"Empty path", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FileExists(tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}

	// Cleanup
	os.Remove(testFile)
}
