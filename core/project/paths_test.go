package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatProjectPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := filepath.Join(home, "sandbox")
	dir := filepath.Join(root, "plist")

	got := FormatPath(dir, root, true, true)
	if got != "plist" {
		t.Fatalf("relative = %q, want plist", got)
	}

	got = FormatPath(dir, root, false, true)
	want := filepath.Join("~", "sandbox", "plist")
	if got != want {
		t.Fatalf("absolute+tilde = %q, want %q", got, want)
	}

	got = FormatPath(dir, root, true, false)
	if got != "plist" {
		t.Fatalf("relative no tilde = %q", got)
	}
}

func TestReplaceHomeWithTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := ReplaceHomeWithTilde(home); got != "~" {
		t.Fatalf("home = %q", got)
	}
	path := filepath.Join(home, "sandbox", "plist")
	want := filepath.Join("~", "sandbox", "plist")
	if got := ReplaceHomeWithTilde(path); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestExpandHomeAndNormalizeRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sandbox := filepath.Join(home, "sandbox")
	if err := os.MkdirAll(sandbox, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ExpandHome("~/sandbox")
	if err != nil {
		t.Fatal(err)
	}
	if got != sandbox {
		t.Fatalf("ExpandHome = %q, want %q", got, sandbox)
	}

	got, err = NormalizeRoot("~/sandbox")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(sandbox)
	if err != nil {
		want = sandbox
	}
	if got != want {
		t.Fatalf("NormalizeRoot = %q, want %q", got, want)
	}
}

func TestDirInsideRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "demo")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := DirInsideRoot(root, child)
	if err != nil {
		t.Fatal(err)
	}
	if got != child && got != mustEval(t, child) {
		t.Fatalf("DirInsideRoot = %q", got)
	}
	if _, err := DirInsideRoot(root, root); err == nil {
		t.Fatal("scan root itself should be rejected")
	}
	outside := t.TempDir()
	if _, err := DirInsideRoot(root, outside); err == nil {
		t.Fatal("directory outside the root should be rejected")
	}
}

func mustEval(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
