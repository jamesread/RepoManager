// Package project discovers Git projects under a scan root and formats
// their paths for display.
package project

import (
	"os"
	"path/filepath"
)

// IsIndexed reports whether path still exists and is a primary Git checkout
// (not a linked worktree or submodule checkout where .git is a file).
func IsIndexed(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return isPrimaryGitCheckout(path)
}

// isPrimaryGitCheckout reports whether dir is a normal repository root.
// Linked worktrees and submodule checkouts use a .git file instead of a dir.
func isPrimaryGitCheckout(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}
