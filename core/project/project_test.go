package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsIndexedProject(t *testing.T) {
	dir := t.TempDir()
	if IsIndexed(dir) {
		t.Fatal("expected false without .git")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsIndexed(dir) {
		t.Fatal("expected git project")
	}
	worktree := filepath.Join(t.TempDir(), "linked")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /tmp/main/.git/worktrees/linked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsIndexed(worktree) {
		t.Fatal("expected false for linked worktree (.git file)")
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if IsIndexed(dir) {
		t.Fatal("expected deleted directory to be gone")
	}
}
