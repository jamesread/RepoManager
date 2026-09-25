package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFind(t *testing.T) {
	root := t.TempDir()
	mustMkdir := func(parts ...string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(append([]string{root}, parts...)...), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir("alpha", ".git")
	mustMkdir("nested", "beta", ".git")
	mustMkdir("nested", "beta", "src")
	mustMkdir("skipme", "node_modules", "pkg", ".git")
	mustMkdir("plain")

	projects, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range projects {
		got[p.Rel] = true
	}
	if !got["alpha"] || !got[filepath.Join("nested", "beta")] {
		t.Fatalf("missing expected projects: %#v", got)
	}
	if got[filepath.Join("skipme", "node_modules", "pkg")] {
		t.Fatalf("should skip node_modules: %#v", got)
	}
	if got["plain"] {
		t.Fatalf("plain dir should not be a project: %#v", got)
	}
}
