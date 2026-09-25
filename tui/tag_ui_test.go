package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestPopulateTagSuggestions(t *testing.T) {
	st := newTestStore(t)
	pathA := testGitProjectDir(t, "a")
	pathB := testGitProjectDir(t, "b")
	if _, err := st.AddTag(pathA, "wip"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTag(pathB, "wip"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTag(pathB, "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTag(pathB, "changes"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTag(pathB, "behind"); err != nil {
		t.Fatal(err)
	}

	m := testWriteModel(t, []list.Item{
		projectItem{path: pathA, rel: "a", tags: []string{"wip", "Git"}},
	})
	m.store = st
	m.populateTagSuggestions([]projectItem{{path: pathA, rel: "a", tags: []string{"wip", "Git"}}})

	if len(m.tagSuggestions) != 1 || m.tagSuggestions[0].value != "done" {
		t.Fatalf("suggestions=%#v", m.tagSuggestions)
	}
	if m.tagSuggestions[0].count != 1 {
		t.Fatalf("count=%d", m.tagSuggestions[0].count)
	}
}

func TestMoveTagSuggestion(t *testing.T) {
	m := testWriteModel(t, nil)
	m.tagSugIdx = -1
	m.tagSuggestions = []valueCount{
		{value: "wip", count: 3},
		{value: "done", count: 1},
	}
	m.moveTagSuggestion(+1)
	if m.tagInput.Value() != "wip" {
		t.Fatalf("value=%q", m.tagInput.Value())
	}
	m.moveTagSuggestion(+1)
	if m.tagInput.Value() != "done" {
		t.Fatalf("value=%q", m.tagInput.Value())
	}
}

func TestVisibleTagSuggestionsFiltersTypedText(t *testing.T) {
	m := testWriteModel(t, nil)
	m.tagSugIdx = -1
	m.tagSuggestions = []valueCount{
		{value: "wip", count: 2},
		{value: "done", count: 1},
		{value: "archived", count: 1},
	}
	m.tagInput.SetValue("do")
	got := m.visibleTagSuggestions()
	if len(got) != 1 || got[0].value != "done" {
		t.Fatalf("got=%#v", got)
	}
}

func TestRenderTagSuggestionsUsesColors(t *testing.T) {
	m := testWriteModel(t, nil)
	m.tagSugIdx = -1
	m.tagSuggestions = []valueCount{{value: "Git", count: 2}}
	out := m.renderTagSuggestions()
	if !strings.Contains(out, "Git") {
		t.Fatalf("output=%q", out)
	}
	if len(out) <= len("Git") {
		t.Fatalf("expected styled tag chip, got %q", out)
	}
}
