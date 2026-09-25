package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

func TestSessionRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "sandbox")
	sess := store.RootSession{
		SelectedPath: "/tmp/proj",
		QuickFilter:  "olive",
		SortFieldID:  "name",
		AdvFilter: []store.FilterRule{
			{FieldID: "tags", Values: []string{"Git", "GitHub"}},
		},
	}
	if err := st.SetSession(root, sess); err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.SessionFor(root)
	if got.SelectedPath != sess.SelectedPath || got.QuickFilter != sess.QuickFilter {
		t.Fatalf("got=%#v", got)
	}
	if len(got.AdvFilter) != 1 || got.AdvFilter[0].FieldID != "tags" {
		t.Fatalf("adv=%#v", got.AdvFilter)
	}
}

func TestRestoreFilterRules(t *testing.T) {
	st := newTestStore(t)

	m := &model{
		store: st,
		root:  t.TempDir(),
		list:  list.New(nil, list.NewDefaultDelegate(), 80, 24),
	}
	attachTestEngine(t, m, st, m.root)

	rules := restoreFilterRules(m, []store.FilterRule{
		{FieldID: "tags", Values: []string{"Git"}},
		{FieldID: "missing.field"},
	})
	if len(rules) != 1 || rules[0].FieldID != "tags" || rules[0].Label != "Tags" {
		t.Fatalf("rules=%#v", rules)
	}
}

func TestRestoreSessionState(t *testing.T) {
	st := newTestStore(t)
	root := filepath.Join(t.TempDir(), "sandbox")
	pathA := testGitProjectDir(t, "alpha")
	pathB := testGitProjectDir(t, "beta")

	m := &model{
		store:     st,
		root:      root,
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		layout:    layoutComfy,
		advFilter: newAdvancedFilter(),
		all: []projectItem{
			{path: pathA, rel: "alpha", tags: []string{"Git"}},
			{path: pathB, rel: "beta", tags: []string{"Git"}},
		},
	}
	attachTestEngine(t, m, st, root)
	m.list.SetFilteringEnabled(true)
	m.advFilter.rules = []filterRule{{FieldID: "tags", Label: "Tags", Values: []string{"Git"}}}
	for _, f := range m.sortableFields() {
		if f.ID != "name" {
			m.setSortFields([]string{f.ID})
			break
		}
	}
	m.activeViewID = viewIDPrefixPreset + "git"
	m.publishList()
	m.list.Select(1)

	if err := st.SetSession(root, m.currentSession()); err != nil {
		t.Fatal(err)
	}

	reloaded := &model{
		store:     st,
		root:      root,
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		layout:    layoutComfy,
		advFilter: newAdvancedFilter(),
		all: []projectItem{
			{path: pathA, rel: "alpha", tags: []string{"Git"}},
			{path: pathB, rel: "beta", tags: []string{"Git"}},
		},
	}
	attachTestEngine(t, reloaded, st, root)
	reloaded.list.SetFilteringEnabled(true)
	reloaded.restoreSession()

	if reloaded.activeViewID != viewIDPrefixPreset+"git" {
		t.Fatalf("view=%q", reloaded.activeViewID)
	}
	if reloaded.currentSort().ID == "name" {
		t.Fatalf("sort=%q", reloaded.currentSort().ID)
	}
	if len(reloaded.advFilter.rules) != 1 {
		t.Fatalf("adv=%#v", reloaded.advFilter.rules)
	}
	if cur, ok := reloaded.list.SelectedItem().(projectItem); !ok || cur.path != pathB {
		t.Fatalf("selected=%#v", reloaded.list.SelectedItem())
	}
}

func TestBootstrapDoesNotOverwriteSession(t *testing.T) {
	st := newTestStore(t)
	root := filepath.Join(t.TempDir(), "sandbox")
	path := testGitProjectDir(t, "alpha")
	if err := st.SetSession(root, store.RootSession{
		SortFieldID:  helpers.FieldDirMtime,
		ActiveViewID: viewIDPrefixPreset + "git",
		AdvFilter: []store.FilterRule{
			{FieldID: "tags", Values: []string{"Git"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_ = st.SetCacheRoot(root, []store.CachedProject{{Path: path, Rel: "alpha"}})

	m := &model{
		store:     st,
		root:      root,
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		advFilter: newAdvancedFilter(),
	}
	m.list.SetFilteringEnabled(true)
	m.lastSession = st.SessionFor(root)
	m.restoringSession = true
	attachTestEngine(t, m, st, root)
	m.restoreSession()

	got := st.SessionFor(root)
	if got.SortFieldID != helpers.FieldDirMtime {
		t.Fatalf("session sort=%q", got.SortFieldID)
	}
	if got.ActiveViewID != viewIDPrefixPreset+"git" {
		t.Fatalf("session view=%q", got.ActiveViewID)
	}
	if len(got.AdvFilter) != 1 {
		t.Fatalf("session adv=%#v", got.AdvFilter)
	}
	if m.currentSort().ID != helpers.FieldDirMtime {
		t.Fatalf("model sort=%q", m.currentSort().ID)
	}
	if m.activeViewID != viewIDPrefixPreset+"git" {
		t.Fatalf("model view=%q", m.activeViewID)
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
