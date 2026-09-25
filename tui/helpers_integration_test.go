package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/jamesread/RepoManager/core/helpers"
)

func TestSyncGitStatusTagsRemovesStale(t *testing.T) {
	st := newTestStore(t)
	path := testGitProjectDir(t, "repo")
	if _, err := st.AddTag(path, helpers.GitTagChanges); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTag(path, helpers.GitTagAhead); err != nil {
		t.Fatal(err)
	}

	m := &model{store: st, root: t.TempDir()}
	attachTestEngine(t, m, st, m.root)
	m.syncGitStatusTags(path, nil)

	tags := st.TagsFor(path)
	for _, tag := range tags {
		if tag == helpers.GitTagChanges || tag == helpers.GitTagAhead || tag == helpers.GitTagBehind {
			t.Fatalf("stale status tag %q still present: %v", tag, tags)
		}
	}
}

func TestRemoveTag(t *testing.T) {
	st := newTestStore(t)
	path := testGitProjectDir(t, "repo")
	if _, err := st.AddTag(path, "wip"); err != nil {
		t.Fatal(err)
	}
	removed, err := st.RemoveTag(path, "wip")
	if err != nil || !removed {
		t.Fatalf("RemoveTag: removed=%v err=%v", removed, err)
	}
	if tags := st.TagsFor(path); len(tags) != 0 {
		t.Fatalf("tags=%v", tags)
	}
}

func TestRefreshTitleGitHubPending(t *testing.T) {
	st := newTestStore(t)
	stub := newStubEngine(st, t.TempDir())
	stub.ghPending = 2
	m := model{
		width:    120,
		root:     stub.root,
		store:    st,
		eng:      stub,
		scanning: true,
		list:     list.New(nil, list.NewDefaultDelegate(), 0, 0),
	}
	m.refreshTitle()
	if !strings.Contains(m.list.Title, "GH 2") {
		t.Fatalf("title=%q", m.list.Title)
	}

	stub.ghPending = 0
	m.refreshTitle()
	if strings.Contains(m.list.Title, "GH") {
		t.Fatalf("expected GH hidden, title=%q", m.list.Title)
	}
}

func TestMatchFilterRuleTopics(t *testing.T) {
	p := projectItem{
		rel:  "demo",
		tags: []string{"Git"},
		meta: map[string]any{helpers.FieldGitHubTopics: []string{"go", "tui"}},
	}
	field := helpers.Field{ID: helpers.FieldGitHubTopics, Kind: helpers.FieldMulti}
	if !matchFilterRule(p, filterRule{FieldID: helpers.FieldGitHubTopics, Value: "tui"}, field) {
		t.Fatal("expected topic match")
	}
	if matchFilterRule(p, filterRule{FieldID: helpers.FieldGitHubTopics, Value: "rust"}, field) {
		t.Fatal("expected no match")
	}
}

func TestMatchFilterRuleOpenIssues(t *testing.T) {
	p := projectItem{
		meta: map[string]any{helpers.FieldGitHubOpenIssues: 5},
	}
	field := helpers.Field{ID: helpers.FieldGitHubOpenIssues, Kind: helpers.FieldInt}
	if !matchFilterRule(p, filterRule{FieldID: helpers.FieldGitHubOpenIssues, Value: "5"}, field) {
		t.Fatal("expected issue count match")
	}
}

func TestModulesHelperCollectAndFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := (helpers.ModulesHelper{}).Collect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	mods, ok := meta[helpers.FieldModules].([]string)
	if !ok || len(mods) != 1 || mods[0] != "Go" {
		t.Fatalf("meta=%#v", meta)
	}

	p := projectItem{rel: "x", meta: meta}
	field := helpers.Field{ID: helpers.FieldModules, Kind: helpers.FieldMulti}
	if !matchFilterRule(p, filterRule{FieldID: helpers.FieldModules, Value: "Go"}, field) {
		t.Fatal("expected Go to match")
	}
	if matchFilterRule(p, filterRule{FieldID: helpers.FieldModules, Value: "npm"}, field) {
		t.Fatal("expected npm not to match")
	}
}
