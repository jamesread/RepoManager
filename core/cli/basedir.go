// Package cli holds small shared command-line helpers for the RepoManager binaries.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/jamesread/RepoManager/core/project"
)

// ParseBasedir reads an optional basedir from args.
// Accepted forms: DIR, --basedir DIR, -basedir DIR, --basedir=DIR.
func ParseBasedir(args []string) (string, error) {
	var basedir string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--basedir" || a == "-basedir":
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a path", a)
			}
			i++
			if basedir != "" {
				return "", fmt.Errorf("basedir specified more than once")
			}
			basedir = args[i]
		case strings.HasPrefix(a, "--basedir="):
			if basedir != "" {
				return "", fmt.Errorf("basedir specified more than once")
			}
			basedir = strings.TrimPrefix(a, "--basedir=")
		case strings.HasPrefix(a, "-"):
			return "", fmt.Errorf("unknown flag: %s", a)
		default:
			if basedir != "" {
				return "", fmt.Errorf("unexpected argument: %s", a)
			}
			basedir = a
		}
	}
	return basedir, nil
}

// ResolveRoot normalizes basedir, defaulting to the current working directory.
func ResolveRoot(basedir string) (string, error) {
	root := basedir
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	return project.NormalizeRoot(root)
}
