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
	mustMkdir("lib", ".compat-worktrees", "8-x-8.5", ".git")
	mustMkdir("plain")
	writeFile := func(path string, content []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir("linked")
	writeFile(filepath.Join("linked", ".git"), []byte("gitdir: /tmp/main/.git/worktrees/linked\n"))

	projects, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range projects {
		got[p.Rel] = true
	}
	if !got["alpha"] || !got["beta"] {
		t.Fatalf("missing expected projects: %#v", got)
	}
	if got["pkg"] {
		t.Fatalf("should skip node_modules: %#v", got)
	}
	if got["plain"] {
		t.Fatalf("plain dir should not be a project: %#v", got)
	}
	if got["8-x-8.5"] {
		t.Fatalf("compat worktree should not be a project: %#v", got)
	}
	if got["linked"] {
		t.Fatalf("linked worktree should not be a project: %#v", got)
	}
}
