package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamesread/RepoManager/core/jsonfile"
	"github.com/jamesread/RepoManager/core/project"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func testGitProjectDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTagColorsArePersistentAndStable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if got := st.ColorForTag("Git"); got != "28" {
		t.Fatalf("Git color = %q, want 28", got)
	}
	updated := st.ColorForTag("updated")
	if updated == "" {
		t.Fatal("expected color for updated")
	}

	st2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := st2.ColorForTag("updated"); got != updated {
		t.Fatalf("updated color changed across runs: %q vs %q", got, updated)
	}
	if got := st2.ColorForTag("Git"); got != "28" {
		t.Fatalf("Git color = %q, want 28", got)
	}
	if got := st2.ColorForTag("GitHub"); got != "55" {
		t.Fatalf("GitHub color = %q, want 55", got)
	}
}

func TestProjectTagsPersist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	path := testGitProjectDir(t, "example-project")
	added, err := st.AddTag(path, "updated")
	if err != nil || !added {
		t.Fatalf("AddTag: added=%v err=%v", added, err)
	}
	st.NoteGitProject(path)
	if err := st.PersistProjectsAndSettings(); err != nil {
		t.Fatal(err)
	}

	st2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	tags := st2.TagsFor(path)
	want := map[string]bool{"updated": true, "Git": true}
	if len(tags) != 2 {
		t.Fatalf("tags = %#v", tags)
	}
	for _, tag := range tags {
		if !want[tag] {
			t.Fatalf("unexpected tag %q in %#v", tag, tags)
		}
	}

	cfg := filepath.Join(home, ".config", "plist", "settings.json")
	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("missing settings file: %v", err)
	}
}

func TestTagOrderPersists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	path := testGitProjectDir(t, "one")
	if _, err := st.AddTag(path, "wip"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTagOrder([]string{"wip", "Git"}); err != nil {
		t.Fatal(err)
	}

	st2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := st2.SortTagsByImportance([]string{"Git", "zzz", "wip"})
	if len(got) != 3 || got[0] != "wip" || got[1] != "Git" || got[2] != "zzz" {
		t.Fatalf("order not persisted: %v", got)
	}
	known := st2.AllKnownTags()
	if len(known) < 2 {
		t.Fatalf("known tags = %v", known)
	}
}

func TestSortTagsByOrder(t *testing.T) {
	tags := []string{"zeta", "Git", "alpha", "wip", "beta"}
	order := []string{"wip", "Git"}
	got := SortTagsByOrder(tags, order)
	want := []string{"wip", "Git", "alpha", "beta", "zeta"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
	// Input must not be mutated.
	if tags[0] != "zeta" {
		t.Fatalf("input mutated: %v", tags)
	}
}

func TestSeedMigratesLegacyGitColor(t *testing.T) {
	colors := map[string]string{"Git": "10"}
	seedDefaultTagColors(colors)
	if colors["Git"] != "28" {
		t.Fatalf("Git not migrated: %q", colors["Git"])
	}
}

func TestPathDisplayDefaults(t *testing.T) {
	st := newTestStore(t)
	if !st.PathRelative() || !st.PathTilde() {
		t.Fatalf("defaults: relative=%v tilde=%v", st.PathRelative(), st.PathTilde())
	}
}

func TestPathDisplayPersist(t *testing.T) {
	st := newTestStore(t)
	if err := st.SetPathRelative(false); err != nil {
		t.Fatal(err)
	}
	if err := st.SetPathTilde(false); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.PathRelative() || reloaded.PathTilde() {
		t.Fatalf("persist failed: relative=%v tilde=%v", reloaded.PathRelative(), reloaded.PathTilde())
	}
}

func TestRecentRootsPersistAndReorder(t *testing.T) {
	st := newTestStore(t)
	older := t.TempDir()
	newer := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")

	if err := st.RememberRoot(older); err != nil {
		t.Fatal(err)
	}
	if err := st.RememberRoot(missing); err != nil {
		t.Fatal(err)
	}
	if err := st.RememberRoot(newer); err != nil {
		t.Fatal(err)
	}
	if err := st.RememberRoot(older); err != nil {
		t.Fatal(err)
	}

	olderNorm, err := project.NormalizeRoot(older)
	if err != nil {
		t.Fatal(err)
	}
	newerNorm, err := project.NormalizeRoot(newer)
	if err != nil {
		t.Fatal(err)
	}
	got := st.RecentRoots()
	if len(got) != 3 || got[0] != olderNorm || got[1] != newerNorm {
		t.Fatalf("recent = %#v", got)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if startup, ok := reloaded.StartupRoot(); !ok || startup != olderNorm {
		t.Fatalf("startup = %q ok=%v", startup, ok)
	}

	if err := os.RemoveAll(older); err != nil {
		t.Fatal(err)
	}
	if startup, ok := reloaded.StartupRoot(); !ok || startup != newerNorm {
		t.Fatalf("startup after removal = %q ok=%v", startup, ok)
	}
}

func TestTagPastelPersists(t *testing.T) {
	st := newTestStore(t)
	first := st.EnsureTagPastels([]string{"wip", "review"})
	if !isPastelHex(first["wip"]) || !isPastelHex(first["review"]) {
		t.Fatalf("colors = %#v", first)
	}
	if first["wip"] == first["review"] {
		t.Fatalf("expected distinct colors, got %s", first["wip"])
	}

	again := st.EnsureTagPastels([]string{"wip"})
	if again["wip"] != first["wip"] {
		t.Fatalf("color changed: %s vs %s", again["wip"], first["wip"])
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	kept := reloaded.EnsureTagPastels([]string{"wip"})
	if kept["wip"] != first["wip"] {
		t.Fatalf("persisted %s, want %s", kept["wip"], first["wip"])
	}

	reloaded.Settings.TagPastels["wip"] = "#ddeeff"
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "plist", "settings.json")
	if err := jsonfile.Write(path, reloaded.Settings); err != nil {
		t.Fatal(err)
	}
	stored, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.EnsureTagPastels([]string{"wip"})["wip"]; got != "#ddeeff" {
		t.Fatalf("stored choice = %s", got)
	}
	if pastelForTagName("Wip") != pastelForTagName("wip") {
		t.Fatal("pastel hash should ignore case")
	}
}

func isPastelHex(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
