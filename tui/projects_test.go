package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/jamesread/RepoManager/core/store"
)

func testGitProjectDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPruneMissingProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	alive := filepath.Join(t.TempDir(), "alive")
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(filepath.Join(alive, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gone, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	st.NoteGitProject(alive)
	st.NoteGitProject(gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	m := &model{
		store: st,
		root:  filepath.Dir(alive),
		list:  list.New(nil, list.NewDefaultDelegate(), 80, 24),
		all: []projectItem{
			{path: alive, rel: "alive", tags: []string{"Git"}},
			{path: gone, rel: "gone", tags: []string{"Git"}},
		},
		found: 2,
	}

	m.pruneMissingProjects()

	if len(m.all) != 1 || m.all[0].path != alive {
		t.Fatalf("all=%#v", m.all)
	}
	if m.found != 1 {
		t.Fatalf("found=%d", m.found)
	}
	if tags := st.TagsFor(gone); len(tags) != 0 {
		t.Fatalf("gone project metadata still present: %#v", tags)
	}
}

func TestStorePruneStaleProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	root := t.TempDir()
	alive := filepath.Join(root, "alive")
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(filepath.Join(alive, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gone, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	st.NoteGitProject(alive)
	st.NoteGitProject(gone)
	if err := st.SetCacheRoot(root, []store.CachedProject{
		{Path: alive, Rel: "alive"},
		{Path: gone, Rel: "gone"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if err := st.PruneStaleProjects(); err != nil {
		t.Fatal(err)
	}
	cached := st.CachedRoot(root)
	if len(cached) != 1 || cached[0].Path != alive {
		t.Fatalf("cache=%#v", cached)
	}
	if tags := st.TagsFor(gone); len(tags) != 0 {
		t.Fatalf("gone metadata still present: %#v", tags)
	}

	store2, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	cached2 := store2.CachedRoot(root)
	if len(cached2) != 1 || cached2[0].Path != alive {
		t.Fatalf("reloaded cache=%#v", cached2)
	}
	if tags := store2.TagsFor(gone); len(tags) != 0 {
		t.Fatalf("reloaded gone metadata: %#v", tags)
	}
}
