// Package project discovers Git projects under a scan root and formats
// their paths for display.
package project

import (
	"os"
	"path/filepath"
)

// IsIndexed reports whether path still exists and contains a .git dir.
func IsIndexed(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false
	}
	return true
}
