package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamesread/RepoManager/tui/widgets"
)

// layoutRowIndex finds the menu row for a setting.
func layoutRowIndex(t *testing.T, m *model, kind layoutMenuKind) int {
	t.Helper()
	for i, r := range m.layoutMenu.rows {
		if r.kind == kind {
			return i
		}
	}
	t.Fatalf("layout menu has no row for kind %d", kind)
	return -1
}

func TestLayoutMenuPathStyleRadio(t *testing.T) {
	m := testTableModel(t)
	if !m.store.PathRelative() {
		t.Fatal("fixture should start relative")
	}

	m.openLayoutMenu()
	idx := layoutRowIndex(t, m, layoutItemPathStyle)
	if m.layoutMenu.rows[idx].radio.SelectedID() != pathStyleRelative {
		t.Fatal("radio should reflect the stored setting on open")
	}
	m.layoutMenu.cursor = idx

	updated, _ := m.updateLayoutMenu(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(model)
	if mm.store.PathRelative() {
		t.Fatal("→ should switch to absolute paths")
	}
	if !strings.Contains(mm.layoutMenuView(), widgets.RadioGlyphOn+" Absolute") {
		t.Fatalf("menu should show Absolute selected:\n%s", mm.layoutMenuView())
	}

	updated, _ = mm.updateLayoutMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	mm = updated.(model)
	if !mm.store.PathRelative() {
		t.Fatal("space should cycle back to relative")
	}

	// j/k are not swallowed by the radio: cursor still moves.
	updated, _ = mm.updateLayoutMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	mm = updated.(model)
	if mm.layoutMenu.cursor != idx-1 {
		t.Fatalf("k should move the cursor, got %d", mm.layoutMenu.cursor)
	}
}

func TestLayoutMenuAllRadioGroups(t *testing.T) {
	m := testTableModel(t)
	m.openLayoutMenu()
	view := m.layoutMenuView()
	if strings.Contains(view, tickMark(true)) || strings.Contains(view, tickMark(false)) {
		t.Fatalf("layout menu should have no tickboxes:\n%s", view)
	}
	for _, title := range []string{"Compact layout", "Layout mode", "Path style", "Path display: ~ for $HOME"} {
		if !strings.Contains(view, title) {
			t.Fatalf("missing row %q:\n%s", title, view)
		}
	}

	// Compact: Off -> On via →.
	m.layoutMenu.cursor = layoutRowIndex(t, m, layoutItemCompact)
	updated, _ := m.updateLayoutMenu(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(model)
	if mm.layout != layoutCompact || mm.store.Layout() != string(layoutCompact) {
		t.Fatalf("compact On should apply and persist, got %s", mm.layout)
	}

	// Layout mode: List -> Table.
	mm.layoutMenu.cursor = layoutRowIndex(t, &mm, layoutItemMode)
	updated, _ = mm.updateLayoutMenu(tea.KeyMsg{Type: tea.KeyRight})
	mm = updated.(model)
	if !mm.tableEnabled() || !mm.store.TableMode() {
		t.Fatal("Table should enable table mode")
	}
	if !strings.Contains(mm.layoutMenuView(), widgets.RadioGlyphOn+" Table") {
		t.Fatalf("menu should show Table selected:\n%s", mm.layoutMenuView())
	}

	// Tilde: flip whatever the fixture has.
	before := mm.store.PathTilde()
	mm.layoutMenu.cursor = layoutRowIndex(t, &mm, layoutItemTilde)
	updated, _ = mm.updateLayoutMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	mm = updated.(model)
	if mm.store.PathTilde() == before {
		t.Fatal("space on the ~ row should flip the setting")
	}

	// Reopening rebuilds rows from settings, so external T/C toggles show.
	mm.layoutMenu.open = false
	mm.toggleTableMode()
	mm.openLayoutMenu()
	i := layoutRowIndex(t, &mm, layoutItemMode)
	if mm.layoutMenu.rows[i].radio.SelectedID() != layoutModeList {
		t.Fatal("reopened menu should reflect table mode off")
	}
}
