package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestScanTagsGit(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "repo", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	eng := New(st, root)
	defer eng.Close()
	events := eng.Subscribe()
	eng.Start()

	var found []Project
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			switch msg := ev.(type) {
			case ProjectFound:
				found = append(found, msg.Project)
			case ScanFinished:
				if msg.Err != "" {
					t.Fatal(msg.Err)
				}
				goto done
			}
		case <-deadline:
			t.Fatal("scan did not finish")
		}
	}
done:
	if len(found) != 1 {
		t.Fatalf("found=%d %#v", len(found), found)
	}
	if len(found[0].Tags) != 1 || found[0].Tags[0] != "Git" {
		t.Fatalf("tags=%#v", found[0].Tags)
	}
}

func TestLoadCachedProjectsSkipsMissing(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	alive := filepath.Join(root, "alive")
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(filepath.Join(alive, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gone, ".git"), 0o755); err != nil {
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

	eng := New(st, root)
	defer eng.Close()
	snap := eng.Snapshot()
	if len(snap) != 1 || snap[0].Path != alive {
		t.Fatalf("snapshot=%#v", snap)
	}
	if tags := st.TagsFor(gone); len(tags) != 0 {
		t.Fatalf("gone metadata still present: %#v", tags)
	}
}

func TestSetRootRescans(t *testing.T) {
	st := newTestStore(t)
	rootA := t.TempDir()
	rootB := t.TempDir()
	pathA := filepath.Join(rootA, "alpha")
	pathB := filepath.Join(rootB, "beta")
	if err := os.MkdirAll(filepath.Join(pathA, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pathB, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	eng := New(st, rootA)
	defer eng.Close()
	events := eng.Subscribe()
	eng.Start()

	waitScan := func() {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case ev := <-events:
				if fin, ok := ev.(ScanFinished); ok {
					if fin.Err != "" {
						t.Fatal(fin.Err)
					}
					return
				}
			case <-deadline:
				t.Fatal("scan did not finish")
			}
		}
	}
	waitScan()

	snap := eng.Snapshot()
	if len(snap) != 1 || snap[0].Path != pathA {
		t.Fatalf("rootA snapshot=%#v", snap)
	}

	if err := eng.Send(context.Background(), SetRootRequest{Root: rootB}); err != nil {
		t.Fatal(err)
	}
	if eng.Root() != rootB {
		t.Fatalf("root=%q want %q", eng.Root(), rootB)
	}
	waitScan()

	snap = eng.Snapshot()
	if len(snap) != 1 || snap[0].Path != pathB {
		t.Fatalf("rootB snapshot=%#v", snap)
	}
}

func TestSetRootRejectsMissing(t *testing.T) {
	st := newTestStore(t)
	eng := New(st, t.TempDir())
	defer eng.Close()
	err := eng.Send(context.Background(), SetRootRequest{Root: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "init", "-b", "main")
	runGit(t, dir, "git", "config", "user.email", "test@example.com")
	runGit(t, dir, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", "README")
	runGit(t, dir, "git", "commit", "-m", "init")
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func TestRefreshRemovesMissingProject(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "gone")
	initGitRepo(t, path)
	st.NoteGitProject(path)
	_ = st.SetCacheRoot(root, []store.CachedProject{{Path: path, Rel: "gone"}})

	eng := New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()

	if len(eng.Snapshot()) != 1 {
		t.Fatalf("snapshot=%#v", eng.Snapshot())
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}

	if err := eng.Send(context.Background(), RefreshRequest{Paths: []string{path}}); err != nil {
		t.Fatal(err)
	}
	if snap := eng.Snapshot(); len(snap) != 0 {
		t.Fatalf("expected empty snapshot after refresh, got %#v", snap)
	}
	if tags := st.TagsFor(path); len(tags) != 0 {
		t.Fatalf("expected store metadata cleared, tags=%#v", tags)
	}
}

func TestRefreshClearsStaleGitStatusTags(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "repo")
	initGitRepo(t, path)

	// Simulate a prior dirty check that left a stale "changes" tag.
	if _, err := st.AddTag(path, helpers.GitTagChanges); err != nil {
		t.Fatal(err)
	}
	st.NoteGitProject(path)
	_ = st.SetCacheRoot(root, []store.CachedProject{{Path: path, Rel: "repo"}})

	eng := New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()

	snap := eng.Snapshot()
	if len(snap) != 1 || !hasTag(snap[0].Tags, helpers.GitTagChanges) {
		t.Fatalf("pre-refresh snapshot=%#v", snap)
	}

	if err := eng.Send(context.Background(), RefreshRequest{Paths: []string{path}}); err != nil {
		t.Fatal(err)
	}

	snap = eng.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot=%#v", snap)
	}
	if hasTag(snap[0].Tags, helpers.GitTagChanges) {
		t.Fatalf("changes tag still present after clean refresh: %#v", snap[0].Tags)
	}
	if hasTag(st.TagsFor(path), helpers.GitTagChanges) {
		t.Fatalf("changes tag still in store: %#v", st.TagsFor(path))
	}
}

func TestAddTagEmitsProjectUpdated(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	st.NoteGitProject(path)
	_ = st.SetCacheRoot(root, []store.CachedProject{{Path: path, Rel: "repo"}})

	eng := New(st, root)
	defer eng.Close()
	events := eng.Subscribe()
	// Drain any enrichment noise before the mutation under test.
drain:
	for {
		select {
		case <-events:
		default:
			break drain
		}
	}

	if err := eng.Send(context.Background(), AddTagRequest{Paths: []string{path}, Tag: "wip"}); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-events:
			upd, ok := ev.(ProjectUpdated)
			if !ok || upd.Project.Path != path {
				continue
			}
			for _, tag := range upd.Project.Tags {
				if tag == "wip" {
					return
				}
			}
			// Ignore enrichment updates that race ahead of the tag mutation.
		case <-deadline:
			t.Fatal("no ProjectUpdated with wip")
		}
	}
}
