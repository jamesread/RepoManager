package main

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	store, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAnnotationsPersistAndClear(t *testing.T) {
	st := newTestStore(t)
	pathA := testGitProjectDir(t, "a")
	pathB := testGitProjectDir(t, "b")

	if err := st.SetAnnotation(pathA, "status", "wip"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAnnotation(pathB, "status", "done"); err != nil {
		t.Fatal(err)
	}
	// Setting a value on a new key registers it globally exactly once.
	if err := st.SetAnnotation(pathA, "owner", "jread"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAnnotation(pathB, "owner", "someone"); err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	keys := reloaded.AnnotationKeys()
	if len(keys) != 2 || keys[0] != "status" || keys[1] != "owner" {
		t.Fatalf("keys=%v", keys)
	}
	if got := reloaded.AnnotationsFor(pathA); got["status"] != "wip" || got["owner"] != "jread" {
		t.Fatalf("a=%v", got)
	}

	// Empty value clears; key stays defined globally.
	if err := reloaded.SetAnnotation(pathA, "status", ""); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.AnnotationsFor(pathA); got["status"] != "" || got["owner"] != "jread" {
		t.Fatalf("after clear a=%v", got)
	}
	if len(reloaded.AnnotationKeys()) != 2 {
		t.Fatalf("keys changed on clear: %v", reloaded.AnnotationKeys())
	}
	// Returned map is a copy.
	got := reloaded.AnnotationsFor(pathB)
	got["status"] = "mutated"
	if reloaded.AnnotationsFor(pathB)["status"] != "done" {
		t.Fatal("AnnotationsFor leaked internal map")
	}
}

func TestAnnotationFieldRoundTrip(t *testing.T) {
	f := annotationField("status")
	if f.ID != "annotation.status" || f.Label != "@status" || !f.Filterable || !f.Sortable {
		t.Fatalf("field=%#v", f)
	}
	key, ok := annotationKeyFromField(f.ID)
	if !ok || key != "status" {
		t.Fatalf("key=%q ok=%v", key, ok)
	}
	if _, ok := annotationKeyFromField("git.last_commit"); ok {
		t.Fatal("non-annotation id parsed as annotation")
	}
	if _, ok := annotationKeyFromField(annotationFieldPrefix); ok {
		t.Fatal("empty key accepted")
	}
}

func TestFieldValueResolvesAnnotations(t *testing.T) {
	p := projectItem{
		rel:         "demo",
		tags:        []string{"Git"},
		annotations: map[string]string{"status": "wip", "empty": ""},
		meta:        map[string]any{helpers.FieldGitHubOwner: "acme"},
	}
	if v := p.fieldValue("annotation.status"); v != "wip" {
		t.Fatalf("status=%v", v)
	}
	if v := p.fieldValue("annotation.missing"); v != nil {
		t.Fatalf("missing should be nil, got %v", v)
	}
	if v := p.fieldValue("annotation.empty"); v != nil {
		t.Fatalf("empty string should be nil, got %v", v)
	}
	if v := p.fieldValue(helpers.FieldGitHubOwner); v != "acme" {
		t.Fatalf("meta passthrough=%v", v)
	}
	if v := p.fieldValue("name"); v != "demo" {
		t.Fatalf("name=%v", v)
	}
	if (projectItem{}).fieldValue("tags") != nil {
		t.Fatal("no tags should be nil")
	}
}

func TestUnsetFilterMatchesMissingAnnotations(t *testing.T) {
	field := annotationField("status")
	rule := filterRule{FieldID: field.ID, Label: field.Label, Unset: true}

	unset := projectItem{rel: "a", annotations: map[string]string{}}
	blank := projectItem{rel: "b", annotations: map[string]string{"status": ""}}
	set := projectItem{rel: "c", annotations: map[string]string{"status": "wip"}}

	if !matchFilterRule(unset, rule, field) {
		t.Fatal("missing annotation should match unset filter")
	}
	if !matchFilterRule(blank, rule, field) {
		t.Fatal("blank annotation should match unset filter")
	}
	if matchFilterRule(set, rule, field) {
		t.Fatal("populated annotation must not match unset filter")
	}

	// Value filter on the same field.
	valueRule := filterRule{FieldID: field.ID, Label: field.Label, Value: "wip"}
	if !matchFilterRule(set, valueRule, field) || matchFilterRule(unset, valueRule, field) {
		t.Fatal("value filter mismatch")
	}
}

func TestUnsetFilterOnHelperField(t *testing.T) {
	field := helpers.Field{ID: helpers.FieldGitHubOwner, Kind: helpers.FieldString}
	rule := filterRule{FieldID: field.ID, Unset: true}
	if !matchFilterRule(projectItem{meta: map[string]any{}}, rule, field) {
		t.Fatal("missing meta should be unset")
	}
	if matchFilterRule(projectItem{meta: map[string]any{helpers.FieldGitHubOwner: "x"}}, rule, field) {
		t.Fatal("present meta should not be unset")
	}
	multi := helpers.Field{ID: helpers.FieldModules, Kind: helpers.FieldMulti}
	if !matchFilterRule(projectItem{meta: map[string]any{helpers.FieldModules: []string{}}}, filterRule{FieldID: helpers.FieldModules, Unset: true}, multi) {
		t.Fatal("empty module list should be unset")
	}
}

func TestAnnotationValueCounts(t *testing.T) {
	st := newTestStore(t)
	paths := map[string]string{
		testGitProjectDir(t, "a"): "wip",
		testGitProjectDir(t, "b"): "done",
		testGitProjectDir(t, "c"): "wip",
		testGitProjectDir(t, "d"): "",
	}
	var pathA string
	for path, v := range paths {
		if v == "wip" && pathA == "" {
			pathA = path
		}
		if err := st.SetAnnotation(path, "status", v); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetAnnotation(pathA, "owner", "jread"); err != nil {
		t.Fatal(err)
	}

	counts := st.AnnotationValueCounts("status")
	if counts["wip"] != 2 || counts["done"] != 1 || len(counts) != 2 {
		t.Fatalf("counts=%v", counts)
	}
	if got := st.AnnotationValueCounts("owner"); got["jread"] != 1 || len(got) != 1 {
		t.Fatalf("owner counts=%v", got)
	}
	if got := st.AnnotationValueCounts("nope"); len(got) != 0 {
		t.Fatalf("unknown key counts=%v", got)
	}
}

func TestRankAndFilterValueCounts(t *testing.T) {
	ranked := rankValueCounts(map[string]int{"done": 1, "Wip": 3, "blocked": 1, "archived": 2})
	want := []string{"Wip", "archived", "blocked", "done"}
	for i, vc := range ranked {
		if vc.value != want[i] {
			t.Fatalf("rank %d = %q, want %q (all=%v)", i, vc.value, want[i], ranked)
		}
	}
	if ranked[0].count != 3 {
		t.Fatalf("top count=%d", ranked[0].count)
	}

	got := filterValueCounts(ranked, "ED")
	if len(got) != 2 || got[0].value != "archived" || got[1].value != "blocked" {
		t.Fatalf("filtered=%v", got)
	}
	if len(filterValueCounts(ranked, "  ")) != len(ranked) {
		t.Fatal("blank filter should keep all")
	}
	if len(filterValueCounts(ranked, "zzz")) != 0 {
		t.Fatal("no match expected")
	}
}

func TestAnnotateSuggestionNavigation(t *testing.T) {
	u := newAnnotateUI()
	u.key = "status"
	u.suggestions = rankValueCounts(map[string]int{"wip": 3, "done": 2, "blocked": 1})
	u.sugIdx = -1

	// First move highlights the top suggestion and fills the input.
	u.moveSuggestion(+1)
	if u.sugIdx != 0 || u.valueIn.Value() != "wip" {
		t.Fatalf("after down: idx=%d value=%q", u.sugIdx, u.valueIn.Value())
	}
	u.moveSuggestion(+1)
	if u.sugIdx != 1 || u.valueIn.Value() != "done" {
		t.Fatalf("after second down: idx=%d value=%q", u.sugIdx, u.valueIn.Value())
	}
	// Wraps from the end back to the start.
	u.moveSuggestion(+1)
	u.moveSuggestion(+1)
	if u.sugIdx != 0 || u.valueIn.Value() != "wip" {
		t.Fatalf("after wrap: idx=%d value=%q", u.sugIdx, u.valueIn.Value())
	}
	// Up from the start wraps to the end.
	u.moveSuggestion(-1)
	if u.sugIdx != 2 || u.valueIn.Value() != "blocked" {
		t.Fatalf("after up-wrap: idx=%d value=%q", u.sugIdx, u.valueIn.Value())
	}

	// Typed text narrows the visible list; first pick anchors on the match.
	u.sugIdx = -1
	u.valueIn.SetValue("do")
	if vis := u.visibleSuggestions(); len(vis) != 1 || vis[0].value != "done" {
		t.Fatalf("visible=%v", vis)
	}
	u.moveSuggestion(+1)
	if u.valueIn.Value() != "done" || u.sugIdx != 1 {
		t.Fatalf("anchored pick: idx=%d value=%q", u.sugIdx, u.valueIn.Value())
	}

	// Empty suggestion set is a no-op.
	empty := newAnnotateUI()
	empty.sugIdx = -1
	empty.moveSuggestion(+1)
	if empty.sugIdx != -1 || empty.valueIn.Value() != "" {
		t.Fatal("moveSuggestion on empty list should not change state")
	}
}

func TestFilterRuleDescribe(t *testing.T) {
	if got := (filterRule{Label: "@status", Unset: true}).describe(); got != "@status is unset" {
		t.Fatalf("got %q", got)
	}
	if got := (filterRule{Label: "@status", Value: "wip"}).describe(); got != "@status=wip" {
		t.Fatalf("got %q", got)
	}
	if got := (filterRule{Label: "Tags", Values: []string{"Git", "GitHub"}}).describe(); got != "Tags=Git+GitHub" {
		t.Fatalf("got %q", got)
	}
	if got := (filterRule{Label: "Tags", Negated: []string{"Archived"}}).describe(); got != "!Tags=Archived" {
		t.Fatalf("got %q", got)
	}
}

func TestAnnotationSummaryOrder(t *testing.T) {
	got := annotationSummary(
		[]string{"status", "owner"},
		map[string]string{"owner": "jread", "zeta": "z", "status": "wip", "blank": ""},
	)
	if got != "@status=wip @owner=jread @zeta=z" {
		t.Fatalf("got %q", got)
	}
	if annotationSummary(nil, nil) != "" {
		t.Fatal("expected empty summary")
	}
}

func TestAnnotationRemovesFromFilteredView(t *testing.T) {
	st := newTestStore(t)
	path := testGitProjectDir(t, "a")

	m := &model{
		store:     st,
		root:      filepath.Join(t.TempDir(), "sandbox"),
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		advFilter: newAdvancedFilter(),
	}
	m.advFilter.rules = []filterRule{{
		FieldID: "annotation.status",
		Label:   "@status",
		Unset:   true,
	}}
	m.all = []projectItem{{
		path:        path,
		rel:         "a",
		annotations: st.AnnotationsFor(path),
	}}
	m.publishList()
	if len(m.list.Items()) != 1 {
		t.Fatalf("want 1 visible, got %d", len(m.list.Items()))
	}
	if err := st.SetAnnotation(path, "status", "wip"); err != nil {
		t.Fatal(err)
	}
	m.refreshProjectAnnotations(path)
	if len(m.list.VisibleItems()) != 0 {
		t.Fatalf("want 0 visible after annotate, got %d", len(m.list.VisibleItems()))
	}
}

func TestAnnotationRemovesFromQuickFilterView(t *testing.T) {
	st := newTestStore(t)
	path := testGitProjectDir(t, "alpha")
	other := testGitProjectDir(t, "beta")

	m := &model{
		store:     st,
		root:      filepath.Join(t.TempDir(), "sandbox"),
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		advFilter: newAdvancedFilter(),
	}
	m.list.SetFilteringEnabled(true)
	m.list.SetFilterText("wip")
	m.all = []projectItem{
		{path: path, rel: "alpha", annotations: map[string]string{"status": "wip"}},
		{path: other, rel: "beta", annotations: map[string]string{"status": "wip"}},
	}
	m.publishList()
	if len(m.list.VisibleItems()) != 2 {
		t.Fatalf("want 2 visible, got %d", len(m.list.VisibleItems()))
	}

	if err := st.SetAnnotation(path, "status", "done"); err != nil {
		t.Fatal(err)
	}
	m.refreshProjectAnnotations(path)
	if m.projectVisibleInView(path) {
		t.Fatal("project should no longer match quick filter")
	}
	if len(m.list.VisibleItems()) != 1 {
		t.Fatalf("want 1 visible, got %d", len(m.list.VisibleItems()))
	}
	if p, ok := m.list.VisibleItems()[0].(projectItem); !ok || p.path != other {
		t.Fatalf("expected beta to remain visible, got %#v", m.list.VisibleItems()[0])
	}
}
