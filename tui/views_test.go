package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

func testModelForViews(t *testing.T) (*model, string) {
	t.Helper()
	st := newTestStore(t)
	root := filepath.Join(t.TempDir(), "sandbox")

	m := &model{
		store:        st,
		root:         root,
		list:         list.New(nil, list.NewDefaultDelegate(), 80, 24),
		width:        120,
		layout:       layoutComfy,
		advFilter:    newAdvancedFilter(),
		lastSession:  store.RootSession{},
		activeViewID: "",
	}
	m.list.SetFilteringEnabled(true)
	attachTestEngine(t, m, st, root)
	return m, root
}

func TestSaveViewReservedName(t *testing.T) {
	m, root := testModelForViews(t)
	if err := m.saveCurrentView("Git repos"); err == nil {
		t.Fatal("expected reserved name error")
	}
	if err := m.saveCurrentView("All projects"); err == nil {
		t.Fatal("expected reserved name error")
	}
	_ = root
}

func TestSaveAndLoadView(t *testing.T) {
	m, _ := testModelForViews(t)
	m.advFilter.rules = []filterRule{{FieldID: "tags", Label: "Tags", Values: []string{"Go"}}}
	m.list.SetFilterText("demo")
	m.setSortFields([]string{helpers.FieldDirMtime, helpers.FieldGitLastCommit})

	if err := m.saveCurrentView("My go view"); err != nil {
		t.Fatal(err)
	}
	if m.activeViewID != savedViewID("My go view") {
		t.Fatalf("active=%q", m.activeViewID)
	}

	m.advFilter.rules = nil
	m.list.ResetFilter()
	m.setSortFields(nil)

	if err := m.loadViewByID(savedViewID("My go view")); err != nil {
		t.Fatal(err)
	}
	if len(m.advFilter.rules) != 1 || m.advFilter.rules[0].Values[0] != "Go" {
		t.Fatalf("rules=%#v", m.advFilter.rules)
	}
	if m.list.FilterValue() != "demo" {
		t.Fatalf("quick filter=%q", m.list.FilterValue())
	}
	if got := m.sortFieldIDs(); len(got) != 2 || got[0] != helpers.FieldDirMtime || got[1] != helpers.FieldGitLastCommit {
		t.Fatalf("sort priority=%v", got)
	}
}

func TestLoadPredefinedView(t *testing.T) {
	m, _ := testModelForViews(t)
	if err := m.loadViewByID(viewIDPrefixPreset + "git"); err != nil {
		t.Fatal(err)
	}
	if m.activeViewTitle() != "Git repos" {
		t.Fatalf("title=%q", m.activeViewTitle())
	}
	if len(m.advFilter.rules) != 1 || m.advFilter.rules[0].Values[0] != "Git" {
		t.Fatalf("rules=%#v", m.advFilter.rules)
	}
}

func TestSessionRestoresActiveView(t *testing.T) {
	m, root := testModelForViews(t)
	if err := m.loadViewByID(viewIDPrefixPreset + "compact"); err != nil {
		t.Fatal(err)
	}
	m.persistSession()

	reloaded := &model{
		store:  m.store,
		root:   root,
		list:   list.New(nil, list.NewDefaultDelegate(), 80, 24),
		layout: parseLayoutMode(m.store.Layout()),
	}
	reloaded.restoreSession()

	if reloaded.activeViewID != viewIDPrefixPreset+"compact" {
		t.Fatalf("active=%q", reloaded.activeViewID)
	}
	if reloaded.layout != layoutCompact {
		t.Fatalf("layout=%q", reloaded.layout)
	}
}

func TestRefreshTitleShowsActiveView(t *testing.T) {
	m, _ := testModelForViews(t)
	m.activeViewID = viewIDPrefixPreset + "recent"
	m.refreshTitle()
	if !strings.Contains(m.titleLeft, "sort ") {
		t.Fatalf("titleLeft=%q", m.titleLeft)
	}
	bar := m.composeTitleBar(m.list.Styles.TitleBar.Render(titleStyle.Render(m.titleLeft)))
	if !strings.Contains(bar, "view: Recently modified") {
		t.Fatalf("title=%q", bar)
	}
}
