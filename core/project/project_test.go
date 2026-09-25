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
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if IsIndexed(dir) {
		t.Fatal("expected deleted directory to be gone")
	}
}
