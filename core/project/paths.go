package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FormatPath renders a project path for display using the user's layout
// preferences. scanRoot is the directory being scanned (plist's cwd argument).
func FormatPath(absPath, scanRoot string, relative, tilde bool) string {
	absPath = filepath.Clean(absPath)
	scanRoot = filepath.Clean(scanRoot)

	display := absPath
	if relative {
		if rel, err := filepath.Rel(scanRoot, absPath); err == nil {
			display = rel
		}
	}

	if tilde {
		display = ReplaceHomeWithTilde(display)
	}
	return display
}

func ReplaceHomeWithTilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	home = filepath.Clean(home)
	if path == home {
		return "~"
	}
	prefix := home + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		return "~" + path[len(home):]
	}
	return path
}

// FormatScanRoot formats the scan root shown beside the list title.
func FormatScanRoot(scanRoot string, tilde bool) string {
	root := filepath.Clean(scanRoot)
	if tilde {
		return ReplaceHomeWithTilde(root)
	}
	return root
}

// NormalizeRoot canonicalizes a scan root for use as a store map key.
func NormalizeRoot(root string) (string, error) {
	root, err := ExpandHome(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root, nil
}

// DirInsideRoot resolves path and returns it when it is a directory strictly
// inside root. The scan root itself and anything outside it are rejected.
func DirInsideRoot(root, path string) (string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rootResolved := rootAbs
	if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootResolved = resolved
	}
	pathResolved := pathAbs
	if resolved, err := filepath.EvalSymlinks(pathAbs); err == nil {
		pathResolved = resolved
	}
	rel, err := filepath.Rel(rootResolved, pathResolved)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("directory is outside the project root")
	}
	info, err := os.Stat(pathResolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory")
	}
	return pathResolved, nil
}

// ExpandHome replaces a leading "~" with the current user's home directory.
func ExpandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
