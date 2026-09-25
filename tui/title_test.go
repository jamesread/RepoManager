package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/helpers"
)

func TestFilterFieldShort(t *testing.T) {
	cases := map[string]string{
		"annotation.status":        "@status",
		"tags":                     "#",
		helpers.FieldGitHubOwner:   "gh.o",
		helpers.FieldGitHubOpenPRs: "gh.pr",
		helpers.FieldModules:       "mods",
	}
	for in, want := range cases {
		if got := filterFieldShort(in); got != want {
			t.Fatalf("%q => %q, want %q", in, got, want)
		}
	}
}

func TestFilterRuleShortSummary(t *testing.T) {
	r := filterRule{FieldID: "annotation.status", Unset: true}
	if got := r.shortSummary(); got != "@status∅" {
		t.Fatalf("got %q", got)
	}
	r = filterRule{FieldID: "tags", Values: []string{"Git", "GitHub"}}
	if got := r.shortSummary(); got != "#=Git+GitHub" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterStatusRaw(t *testing.T) {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetFilteringEnabled(true)
	l.SetFilterText("olive")

	m := model{
		list: l,
		advFilter: advancedFilter{
			rules: []filterRule{
				{FieldID: "tags", Values: []string{"Git"}},
			},
		},
	}
	got := m.filterStatusRaw()
	if !strings.Contains(got, "/olive") || !strings.Contains(got, "#=Git") {
		t.Fatalf("got %q", got)
	}
}

func TestListPaginationSummary(t *testing.T) {
	st := newTestStore(t)
	delegate := newItemDelegate(st, nil, layoutComfy, newRefreshState(), t.TempDir(), nil)
	items := make([]list.Item, 25)
	for i := range items {
		items[i] = projectItem{path: "/p", rel: "proj"}
	}
	l := list.New(items, delegate, 80, 24)
	l.SetShowPagination(true)
	m := model{list: l, store: st, layout: layoutComfy, width: 80, height: 24}
	got := m.listPaginationSummary()
	want := fmt.Sprintf("page 1 of %d · 25 projects", m.list.Paginator.TotalPages)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestListEmptyMessage(t *testing.T) {
	l := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := model{list: l, width: 80, height: 24}
	block := m.listEmptyBlock(80, 10)
	if !strings.Contains(block, listEmptyMessage) {
		t.Fatalf("expected %q in block:\n%s", listEmptyMessage, block)
	}
	view := m.mainListView()
	if strings.Contains(view, "No items") {
		t.Fatalf("should replace default empty text:\n%s", view)
	}
	if !strings.Contains(view, listEmptyMessage) {
		t.Fatalf("expected empty message in view:\n%s", view)
	}
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	var msgLine int = -1
	for i, line := range lines {
		if strings.Contains(line, listEmptyMessage) {
			msgLine = i
			break
		}
	}
	if msgLine < 0 {
		t.Fatal("empty message line not found")
	}
	regions := m.listViewRegions()
	if msgLine < regions.itemsStart || msgLine >= regions.itemsEnd {
		t.Fatalf("message at line %d outside items region %d..%d", msgLine, regions.itemsStart, regions.itemsEnd)
	}
	// Rough vertical centering: message should not hug the top of the items area.
	if msgLine-regions.itemsStart < 2 && regions.itemsEnd-regions.itemsStart > 4 {
		t.Fatalf("expected vertically centered message, got line %d in region %d..%d", msgLine, regions.itemsStart, regions.itemsEnd)
	}
}

func TestTruncateTitleSection(t *testing.T) {
	long := "one · two · three · four · five · six"
	got := truncateTitleSection(long, 20)
	if lipgloss.Width(got) > 20 {
		t.Fatalf("width=%d text=%q", lipgloss.Width(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis, got %q", got)
	}
}

func TestRefreshTitleFilterBetweenPathAndSort(t *testing.T) {
	st := newTestStore(t)

	m := model{
		width: 120,
		root:  "/tmp/sandbox",
		store: st,
		list:  list.New(nil, list.NewDefaultDelegate(), 0, 0),
		advFilter: advancedFilter{
			rules: []filterRule{
				{FieldID: "annotation.status", Label: "@status", Value: "wip"},
			},
		},
	}
	m.list.SetFilteringEnabled(true)
	m.list.SetFilterText("olive")
	m.refreshTitle()

	title := m.list.Title
	pathIdx := strings.Index(title, "sandbox")
	filterIdx := strings.Index(title, "/olive")
	sortIdx := strings.Index(title, "sort ")
	if pathIdx < 0 || filterIdx < pathIdx || sortIdx < filterIdx {
		t.Fatalf("order wrong: %q", title)
	}
	if !strings.Contains(title, "@status=wip") {
		t.Fatalf("missing adv filter: %q", title)
	}
}

func TestRefreshTitleFilterWidthCap(t *testing.T) {
	st := newTestStore(t)

	m := model{
		width: 60,
		root:  "/tmp/sandbox",
		store: st,
		list:  list.New(nil, list.NewDefaultDelegate(), 0, 0),
		advFilter: advancedFilter{
			rules: []filterRule{
				{FieldID: "tags", Values: []string{"Git", "GitHub", "Go", "Rust", "Python"}},
				{FieldID: "annotation.status", Value: "work-in-progress-long-name"},
			},
		},
	}
	m.refreshTitle()

	sortMarker := " · sort "
	sortIdx := strings.Index(m.list.Title, sortMarker)
	if sortIdx < 0 {
		t.Fatalf("title=%q", m.list.Title)
	}
	prefix := m.list.Title[:sortIdx]
	lastSep := strings.LastIndex(prefix, " · ")
	if lastSep < 0 {
		t.Fatal("expected filter segment")
	}
	filterPart := prefix[lastSep+len(" · "):]
	if lipgloss.Width(filterPart) > m.width/2 {
		t.Fatalf("filter too wide: %d > %d (%q)", lipgloss.Width(filterPart), m.width/2, filterPart)
	}
}
