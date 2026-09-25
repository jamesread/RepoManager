package helpers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v (%s)", args, err, out)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "git", "init", "-b", "main")
	runGit(t, dir, "git", "config", "user.email", "test@example.com")
	runGit(t, dir, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", "README")
	runGit(t, dir, "git", "commit", "-m", "init")
}

func TestGitHelperCollect(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	meta, err := (GitHelper{}).Collect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := meta[FieldGitLastCommit]; !ok {
		t.Fatalf("git meta = %#v", meta)
	}
	if tags, ok := meta[MetaKeyTags].([]string); !ok || len(tags) != 0 {
		t.Fatalf("tags = %#v", meta[MetaKeyTags])
	}
}

func TestGitStatusTagsDirty(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tags, err := gitStatusTags(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != GitTagChanges {
		t.Fatalf("tags=%v", tags)
	}
}

func TestGitStatusTagsAhead(t *testing.T) {
	bareParent := t.TempDir()
	dir := t.TempDir()
	bare := filepath.Join(bareParent, "origin.git")
	runGit(t, bareParent, "git", "init", "--bare", "-b", "main", "origin.git")
	initGitRepo(t, dir)
	runGit(t, dir, "git", "remote", "add", "origin", bare)
	runGit(t, dir, "git", "push", "-u", "origin", "main")
	if err := os.WriteFile(filepath.Join(dir, "next.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", "next.txt")
	runGit(t, dir, "git", "commit", "-m", "ahead")

	tags, err := gitStatusTags(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != GitTagAhead {
		t.Fatalf("tags=%v", tags)
	}
}
