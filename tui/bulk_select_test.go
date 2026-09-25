package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/muesli/termenv"
)

func testBulkModel(t *testing.T, items []list.Item) *model {
	t.Helper()
	m := testMouseModel(t, items, layoutComfy)
	m.bulk = newBulkSelectState()
	m.list.SetDelegate(newItemDelegate(m.store, func() []helpers.Field { return m.helperFields() }, m.layout, newRefreshState(), t.TempDir(), m.bulk))
	return m
}

func TestToggleBulkSelect(t *testing.T) {
	m := testBulkModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
		projectItem{path: "/tmp/b", rel: "b"},
	})
	m.list.Select(0)

	cmd := m.toggleBulkSelect()
	if !m.bulk.active {
		t.Fatal("bulk select should be active")
	}
	if !m.bulk.isSelected("/tmp/a") {
		t.Fatal("cursor item should be selected on enter")
	}
	if cmd == nil {
		t.Fatal("expected status command")
	}
}

func TestBulkOperationTargets(t *testing.T) {
	m := testBulkModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
		projectItem{path: "/tmp/b", rel: "b"},
	})
	m.all = []projectItem{
		{path: "/tmp/a", rel: "a"},
		{path: "/tmp/b", rel: "b"},
	}
	m.bulk.active = true
	m.bulk.toggle("/tmp/a")
	m.bulk.toggle("/tmp/b")

	targets := m.operationTargets()
	if len(targets) != 2 {
		t.Fatalf("targets=%d", len(targets))
	}
}

func TestBulkSpaceToggle(t *testing.T) {
	m := testBulkModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
	})
	m.bulk.active = true
	m.list.Select(0)

	cmd, handled := m.handleBulkToggleKey(tea.KeyMsg{Type: tea.KeySpace})
	if !handled || cmd == nil {
		t.Fatal("space should toggle bulk item")
	}
	if !m.bulk.isSelected("/tmp/a") {
		t.Fatal("item should be selected")
	}

	cmd, handled = m.handleBulkToggleKey(tea.KeyMsg{Type: tea.KeySpace})
	if !handled || cmd == nil {
		t.Fatal("space should toggle bulk item off")
	}
	if m.bulk.isSelected("/tmp/a") {
		t.Fatal("item should be deselected")
	}
}

// renderRows renders each visible item through the given delegate and
// returns the raw (ANSI-bearing) output per index.
func renderRows(d itemDelegate, l list.Model) []string {
	items := l.VisibleItems()
	out := make([]string, 0, len(items))
	for i, it := range items {
		var b strings.Builder
		d.Render(&b, l, i, it)
		out = append(out, b.String())
	}
	return out
}

// TestBulkSelectTickboxAllLayouts builds the delegate before bulk select is
// turned on (as startup does) and then toggles it on a copy of the model, the
// way Bubble Tea's by-value Update loop does. The delegate must still see the
// change in every layout.
func TestBulkSelectTickboxAllLayouts(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	const bold = "\x1b[1"
	const backgroundSGR = "\x1b[48;"

	for _, tc := range []struct {
		name   string
		layout layoutMode
		table  bool
	}{
		{"comfy", layoutComfy, false},
		{"compact", layoutCompact, false},
		{"table", layoutComfy, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No tags: chips carry their own background, which would
			// mask the "no row background" check below.
			items := []list.Item{
				projectItem{path: "/tmp/a", rel: "a"},
				projectItem{path: "/tmp/b", rel: "b"},
			}
			m := testBulkModel(t, items)
			m.table = &tableState{}
			m.applyLayout(tc.layout)
			if tc.table {
				m.setTableMode(true) // republishes from the (empty) project set
			}
			m.list.SetItems(items)
			m.list.Select(0)

			// Delegate captured before bulk mode, as at startup.
			d := newItemDelegate(m.store, func() []helpers.Field { return m.helperFields() }, m.layout, newRefreshState(), m.root, m.bulk)
			d.table = m.table

			// Toggle on a value copy, mirroring Update's semantics.
			copied := *m
			copied.toggleBulkSelect() // picks the cursor row (a)
			copied.list.Select(1)     // move cursor off it
			copied.recomputeTableColumns()

			rows := renderRows(d, copied.list)
			if len(rows) != 2 {
				t.Fatalf("expected 2 rows, got %d", len(rows))
			}
			if !strings.Contains(rows[0], tickMark(true)) {
				t.Fatalf("picked row should show a ticked box: %q", rows[0])
			}
			if !strings.Contains(rows[1], tickMark(false)) {
				t.Fatalf("unpicked row should show an empty box: %q", rows[1])
			}
			if !strings.Contains(rows[0], bold) {
				t.Fatalf("picked row should be bold: %q", rows[0])
			}
			for i, row := range rows {
				if strings.Contains(row, backgroundSGR) {
					t.Fatalf("row %d must not set a background: %q", i, row)
				}
			}
		})
	}
}

func TestPopulateTagSuggestionsBulk(t *testing.T) {
	st := newTestStore(t)
	pathA := testGitProjectDir(t, "a")
	pathB := testGitProjectDir(t, "b")
	if _, err := st.AddTag(pathA, "wip"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTag(pathB, "done"); err != nil {
		t.Fatal(err)
	}

	m := testBulkModel(t, nil)
	m.store = st
	items := []projectItem{
		{path: pathA, rel: "a", tags: []string{"wip", "Git"}},
		{path: pathB, rel: "b", tags: []string{"Git", "done"}},
	}
	m.populateTagSuggestions(items)
	for _, vc := range m.tagSuggestions {
		if vc.value == "Git" {
			t.Fatalf("Git should be excluded when all projects have it: %#v", vc)
		}
	}
	foundWip := false
	foundDone := false
	for _, vc := range m.tagSuggestions {
		switch vc.value {
		case "wip":
			foundWip = true
		case "done":
			foundDone = true
		}
	}
	if !foundWip || !foundDone {
		t.Fatalf("expected wip and done suggestions, got %#v", m.tagSuggestions)
	}
}
