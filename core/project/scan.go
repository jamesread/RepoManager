package project

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Found is a project discovered by Scan: the directory holding .git and its
// path relative to the scan root.
type Found struct {
	Path string
	Rel  string
}

// skipDirs are never descended into; they hold dependencies or build output
// rather than the user's own projects.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, ".cache": true, "target": true, "dist": true, "build": true,
}

// Scan walks root and calls onFound once for each directory that contains a
// .git entry. Nested repositories below a found project are not descended
// into.
func Scan(root string, onFound func(Found)) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		name := d.Name()

		if d.IsDir() {
			if skipDirs[name] {
				return fs.SkipDir
			}
		}

		if name != ".git" {
			return nil
		}

		projectDir := filepath.Dir(path)
		if _, ok := seen[projectDir]; ok {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		seen[projectDir] = struct{}{}

		rel, relErr := filepath.Rel(root, projectDir)
		if relErr != nil {
			rel = projectDir
		}
		if rel == "." {
			rel = filepath.Base(projectDir) + " (cwd)"
		}

		onFound(Found{Path: projectDir, Rel: rel})

		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
}

// Find returns every project under root, ordered by relative path
// (case-insensitively).
func Find(root string) ([]Found, error) {
	var projects []Found
	err := Scan(root, func(p Found) { projects = append(projects, p) })
	if err != nil {
		return nil, err
	}
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Rel) < strings.ToLower(projects[j].Rel)
	})
	return projects, nil
}
