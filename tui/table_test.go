package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/jamesread/RepoManager/core/helpers"
)

func testTableModel(t *testing.T) *model {
	t.Helper()
	m := testSortModel(t)
	m.width, m.height = 120, 30
	m.table = &tableState{}
	m.applyLayout(layoutComfy)
	m.resizeList(118, 26)
	m.publishListNow()
	return m
}

func TestTableModeToggleAndHeader(t *testing.T) {
	m := testTableModel(t)
	if m.tableEnabled() {
		t.Fatal("table mode should start off")
	}
	regions := m.listViewRegions()
	if regions.headerLines != 0 {
		t.Fatalf("headerLines=%d", regions.headerLines)
	}

	m.setTableMode(true)
	if !m.tableEnabled() || !m.store.TableMode() {
		t.Fatal("table mode should be on and persisted")
	}
	if h, sp := m.itemDelegateSize(); h != 1 || sp != 0 {
		t.Fatalf("delegate size=(%d,%d)", h, sp)
	}

	cols := m.table.columns
	ids := make([]string, 0, len(cols))
	for _, c := range cols {
		ids = append(ids, c.id)
	}
	joined := strings.Join(ids, ",")
	for _, want := range []string{"name", tableColPath, helpers.FieldDirMtime, helpers.FieldGitHubOwner} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing column %q in %s", want, joined)
		}
	}
	if strings.Contains(joined, helpers.FieldGitHubRepo) {
		t.Fatalf("empty column should be omitted: %s", joined)
	}

	header := ansi.Strip(m.tableHeaderLine())
	if !strings.Contains(header, "Name") || !strings.Contains(header, "GitHub owner") {
		t.Fatalf("header=%q", header)
	}
	if !strings.Contains(header, "Name "+sortGlyphAsc) {
		t.Fatalf("primary sort should be marked ascending: %q", header)
	}

	view := ansi.Strip(m.mainListView())
	lines := strings.Split(view, "\n")
	regions = m.listViewRegions()
	if regions.headerLines != 1 || regions.itemsStart != regions.titleLines+1 {
		t.Fatalf("regions=%+v", regions)
	}
	if !strings.Contains(lines[regions.titleLines], "GitHub owner") {
		t.Fatalf("header line missing from view: %q", lines[regions.titleLines])
	}
	// Row values appear as bare values, not "Label: value".
	row := lines[regions.itemsStart]
	if strings.Contains(row, "GitHub owner:") {
		t.Fatalf("row should not contain label prefix: %q", row)
	}
	if !strings.Contains(row, "acme") && !strings.Contains(row, "zed") {
		t.Fatalf("row should contain owner value: %q", row)
	}

	m.setTableMode(false)
	if m.tableEnabled() || m.store.TableMode() {
		t.Fatal("table mode should be off")
	}
}

func TestTableHeaderClickSorts(t *testing.T) {
	m := testTableModel(t)
	m.setTableMode(true)

	var ownerCol tableColumn
	for _, c := range m.table.columns {
		if c.id == helpers.FieldGitHubOwner {
			ownerCol = c
		}
	}
	if ownerCol.width == 0 {
		t.Fatal("owner column not found")
	}
	x := tableRowPrefix
	for _, c := range m.table.columns {
		if c.id == helpers.FieldGitHubOwner {
			break
		}
		x += c.width + tableColGap
	}

	regions := m.listViewRegions()
	click := tea.MouseMsg{
		X:      x + 1,
		Y:      regions.titleLines + listViewTopOffset,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	_, _, handled := m.handleMouse(click)
	if !handled {
		t.Fatal("header click should be handled")
	}
	if k := m.currentSortKey(); k.Field.ID != helpers.FieldGitHubOwner || k.Desc {
		t.Fatalf("first click should sort owner ascending, got %+v", k)
	}
	if !strings.Contains(ansi.Strip(m.tableHeaderLine()), "GitHub owner "+sortGlyphAsc) {
		t.Fatalf("header should mark owner ascending: %q", ansi.Strip(m.tableHeaderLine()))
	}
	if got := strings.Join(visibleRels(m), ","); got != "b,c,a" {
		t.Fatalf("owner asc: %s", got)
	}

	// Clicking the primary column again flips direction.
	_, _, _ = m.handleMouse(click)
	if k := m.currentSortKey(); k.Field.ID != helpers.FieldGitHubOwner || !k.Desc {
		t.Fatalf("second click should flip to descending, got %+v", k)
	}
	if !strings.Contains(ansi.Strip(m.tableHeaderLine()), "GitHub owner "+sortGlyphDesc) {
		t.Fatalf("header should mark owner descending: %q", ansi.Strip(m.tableHeaderLine()))
	}
	if got := strings.Join(visibleRels(m), ","); got != "a,b,c" {
		t.Fatalf("owner desc: %s", got)
	}

	// Path is not sortable: clicking it leaves the sort alone.
	_, _, _ = m.handleMouse(tea.MouseMsg{
		X:      tableRowPrefix + m.table.columns[0].width + tableColGap + 1,
		Y:      regions.titleLines + listViewTopOffset,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if m.currentSort().ID != helpers.FieldGitHubOwner {
		t.Fatalf("path click changed sort to %q", m.currentSort().ID)
	}
}

func TestFitTableColumns(t *testing.T) {
	cols := []tableColumn{
		{id: "name", width: 40},
		{id: "a", width: 40},
		{id: "b", width: 40},
	}
	fitted := fitTableColumns(cols, 60)
	if got := tableColumnsWidth(fitted); got > 60 {
		t.Fatalf("width=%d", got)
	}
	if fitted[0].id != "name" {
		t.Fatal("name must be kept")
	}

	tiny := fitTableColumns([]tableColumn{{id: "name", width: 40}, {id: "a", width: 40}}, 10)
	if len(tiny) != 1 || tiny[0].id != "name" {
		t.Fatalf("expected only name, got %+v", tiny)
	}
}

func TestTableQuickToggleKey(t *testing.T) {
	m := testTableModel(t)
	keyT := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}
	if _, handled := m.handleAppKeys(keyT); !handled || !m.tableEnabled() {
		t.Fatal("T should enable table mode")
	}
	if !m.store.TableMode() {
		t.Fatal("table mode should persist")
	}
	if _, handled := m.handleAppKeys(keyT); !handled || m.tableEnabled() {
		t.Fatal("T again should disable table mode")
	}
}

func TestCompactQuickToggleKey(t *testing.T) {
	m := testTableModel(t)
	keyC := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}}
	if _, handled := m.handleAppKeys(keyC); !handled || m.layout != layoutCompact {
		t.Fatalf("C should switch to compact, got %s", m.layout)
	}
	if m.store.Layout() != string(layoutCompact) {
		t.Fatal("compact layout should persist")
	}
	if _, handled := m.handleAppKeys(keyC); !handled || m.layout != layoutComfy {
		t.Fatalf("C again should switch back to comfy, got %s", m.layout)
	}

	// Still applies while table mode is on.
	m.setTableMode(true)
	if _, handled := m.handleAppKeys(keyC); !handled || m.layout != layoutCompact {
		t.Fatalf("C should still toggle compact in table mode, got %s", m.layout)
	}
	if !m.tableEnabled() {
		t.Fatal("toggling compact must not leave table mode")
	}
}

func TestTagOrderRebindToO(t *testing.T) {
	m := testTableModel(t)
	if _, handled := m.handleAppKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}); !handled || !m.tagOrder.open {
		t.Fatal("O should open tag order")
	}
	m.tagOrder.open = false
	if _, handled := m.handleAppKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}); !handled || m.tagOrder.open {
		t.Fatal("T should not open tag order")
	}
	if _, handled := m.handleAppKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}); !handled || !m.sortMenu.open {
		t.Fatal("S should open sort menu")
	}
}
