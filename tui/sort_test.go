package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

func testSortModel(t *testing.T) *model {
	t.Helper()
	m, _ := testModelForViews(t)
	m.height = 40
	m.list.SetShowStatusBar(false)
	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-time.Hour)
	m.all = []projectItem{
		{path: testGitProjectDir(t, "b"), rel: "b", meta: map[string]any{helpers.FieldDirMtime: old, helpers.FieldGitHubOwner: "acme"}},
		{path: testGitProjectDir(t, "a"), rel: "a", meta: map[string]any{helpers.FieldDirMtime: old, helpers.FieldGitHubOwner: "zed"}},
		{path: testGitProjectDir(t, "c"), rel: "c", meta: map[string]any{helpers.FieldDirMtime: recent, helpers.FieldGitHubOwner: "acme"}},
	}
	return m
}

func visibleRels(m *model) []string {
	var out []string
	for _, it := range m.list.VisibleItems() {
		out = append(out, it.(projectItem).rel)
	}
	return out
}

func TestMultiFieldSortPriority(t *testing.T) {
	m := testSortModel(t)

	// Ascending owner, then ascending mtime (oldest first).
	m.setSortFields([]string{helpers.FieldGitHubOwner, helpers.FieldDirMtime})
	m.publishListNow()
	if got := strings.Join(visibleRels(m), ","); got != "b,c,a" {
		t.Fatalf("owner asc then mtime asc: %s", got)
	}

	// Ascending owner, then descending mtime (newest first).
	m.setSortSpecs([]store.SortSpec{{ID: helpers.FieldGitHubOwner}, {ID: helpers.FieldDirMtime, Desc: true}})
	m.publishListNow()
	if got := strings.Join(visibleRels(m), ","); got != "c,b,a" {
		t.Fatalf("owner asc then mtime desc: %s", got)
	}

	// Descending mtime first, then ascending owner.
	m.setSortSpecs([]store.SortSpec{{ID: helpers.FieldDirMtime, Desc: true}, {ID: helpers.FieldGitHubOwner}})
	m.publishListNow()
	if got := strings.Join(visibleRels(m), ","); got != "c,b,a" {
		t.Fatalf("mtime desc then owner asc: %s", got)
	}

	// Ties on owner fall back to name ascending, even when owner is descending.
	m.setSortSpecs([]store.SortSpec{{ID: helpers.FieldGitHubOwner, Desc: true}})
	m.publishListNow()
	if got := strings.Join(visibleRels(m), ","); got != "a,b,c" {
		t.Fatalf("owner desc then name: %s", got)
	}

	// Name descending reverses the default order.
	m.setSortSpecs([]store.SortSpec{{ID: "name", Desc: true}})
	m.publishListNow()
	if got := strings.Join(visibleRels(m), ","); got != "c,b,a" {
		t.Fatalf("name desc: %s", got)
	}
}

func TestMissingValuesSortLastBothDirections(t *testing.T) {
	m := testSortModel(t)
	m.all = append(m.all, projectItem{path: testGitProjectDir(t, "d"), rel: "d", meta: map[string]any{}})
	for _, desc := range []bool{false, true} {
		m.setSortSpecs([]store.SortSpec{{ID: helpers.FieldGitHubOwner, Desc: desc}})
		m.publishListNow()
		rels := visibleRels(m)
		if rels[len(rels)-1] != "d" {
			t.Fatalf("desc=%v: missing value should sort last, got %v", desc, rels)
		}
	}
}

func TestTimeFieldsDefaultToNewestFirst(t *testing.T) {
	m := testSortModel(t)
	m.toggleSortField(helpers.FieldDirMtime)
	if k := m.currentSortKey(); k.Field.ID != helpers.FieldDirMtime || !k.Desc {
		t.Fatalf("expected mtime desc by default, got %+v", k)
	}
	m.setSortSpecs(nil)
	m.setPrimarySort(helpers.FieldGitHubOwner)
	if k := m.currentSortKey(); k.Desc {
		t.Fatalf("string fields should default ascending, got %+v", k)
	}
	if !strings.Contains(m.sortTitleLabel(), sortGlyphAsc) {
		t.Fatalf("title should carry direction glyph: %q", m.sortTitleLabel())
	}
}

func TestSortMenuDirectionToggle(t *testing.T) {
	m := testSortModel(t)
	m.openSortMenu()
	// Select the first entry (Name), then toggle its direction with `d`.
	updated, _ := m.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	mm := updated.(model)
	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	mm = updated.(model)
	if k := mm.currentSortKey(); k.Field.ID != "name" || !k.Desc {
		t.Fatalf("expected name desc, got %+v", k)
	}
	view := mm.sortMenuView()
	if !strings.Contains(view, "Z to A") {
		t.Fatalf("menu should describe direction: %s", view)
	}
	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyRight})
	mm = updated.(model)
	if k := mm.currentSortKey(); k.Desc {
		t.Fatalf("right arrow should flip back to asc, got %+v", k)
	}
}

func TestSessionPersistsSortDirection(t *testing.T) {
	m, root := testModelForViews(t)
	m.setSortSpecs([]store.SortSpec{{ID: helpers.FieldDirMtime, Desc: true}, {ID: helpers.FieldGitHubOwner}})
	m.persistSession()

	reloaded := &model{
		store:     m.store,
		root:      root,
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		advFilter: newAdvancedFilter(),
	}
	attachTestEngine(t, reloaded, m.store, root)
	reloaded.restoreSession()
	got := reloaded.effectiveSortSpecs()
	if len(got) != 2 || got[0] != (store.SortSpec{ID: helpers.FieldDirMtime, Desc: true}) || got[1] != (store.SortSpec{ID: helpers.FieldGitHubOwner}) {
		t.Fatalf("restored specs=%+v", got)
	}
}

func TestRecentPresetIsNewestFirst(t *testing.T) {
	m := testSortModel(t)
	if err := m.loadViewByID(viewIDPrefixPreset + "recent"); err != nil {
		t.Fatal(err)
	}
	if k := m.currentSortKey(); k.Field.ID != helpers.FieldDirMtime || !k.Desc {
		t.Fatalf("recent preset should be mtime desc, got %+v", k)
	}
	if got := visibleRels(m); got[0] != "c" {
		t.Fatalf("newest first expected, got %v", got)
	}
}

func TestSortTitleLabel(t *testing.T) {
	m := testSortModel(t)
	if got := m.sortTitleLabel(); got != "Name"+sortGlyphAsc {
		t.Fatalf("default label=%q", got)
	}
	m.setSortFields([]string{helpers.FieldDirMtime, helpers.FieldGitHubOwner, "bogus.field"})
	if got := m.sortTitleLabel(); !strings.HasSuffix(got, sortGlyphAsc+" +1") {
		t.Fatalf("label=%q", got)
	}
	if got := m.sortFieldIDs(); len(got) != 2 {
		t.Fatalf("unknown field should be dropped: %v", got)
	}
}

func TestSortMenuToggleAndReorder(t *testing.T) {
	m := testSortModel(t)
	m.openSortMenu()
	if !m.sortMenu.open {
		t.Fatal("menu should open")
	}

	// Default entries: nothing selected; space on the first entry selects it.
	entries := m.sortMenuEntries()
	if len(entries) == 0 || entries[0].selected {
		t.Fatalf("entries=%+v", entries)
	}
	first := entries[0].field.ID

	updated, _ := m.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	mm := updated.(model)
	if got := mm.sortFieldIDs(); len(got) != 1 || got[0] != first {
		t.Fatalf("after select: %v", got)
	}

	// Move down to the next unselected entry and select it as second priority.
	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = updated.(model)
	entries = mm.sortMenuEntries()
	second := entries[mm.sortMenu.cursor].field.ID
	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	mm = updated.(model)
	if got := mm.sortFieldIDs(); len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("after second select: %v", got)
	}

	// K raises the cursor field (second) to primary.
	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	mm = updated.(model)
	if got := mm.sortFieldIDs(); got[0] != second || got[1] != first {
		t.Fatalf("after raise: %v", got)
	}
	if mm.sortMenu.cursor != 0 {
		t.Fatalf("cursor should follow moved field, got %d", mm.sortMenu.cursor)
	}

	// Space deselects the cursor field.
	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	mm = updated.(model)
	if got := mm.sortFieldIDs(); len(got) != 1 || got[0] != first {
		t.Fatalf("after deselect: %v", got)
	}

	updated, _ = mm.updateSortMenu(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(model)
	if mm.sortMenu.open {
		t.Fatal("esc should close")
	}
}

func TestSessionPersistsSortPriority(t *testing.T) {
	m, root := testModelForViews(t)
	m.setSortFields([]string{helpers.FieldDirMtime, helpers.FieldGitHubOwner})
	m.persistSession()

	reloaded := &model{
		store:     m.store,
		root:      root,
		list:      list.New(nil, list.NewDefaultDelegate(), 80, 24),
		advFilter: newAdvancedFilter(),
	}
	attachTestEngine(t, reloaded, m.store, root)
	reloaded.restoreSession()
	if got := reloaded.sortFieldIDs(); len(got) != 2 || got[0] != helpers.FieldDirMtime || got[1] != helpers.FieldGitHubOwner {
		t.Fatalf("restored sort=%v", got)
	}
}

func TestLegacySingleSortFieldRestores(t *testing.T) {
	m, root := testModelForViews(t)
	if err := m.store.SetSession(root, store.RootSession{SortFieldID: helpers.FieldDirMtime}); err != nil {
		t.Fatal(err)
	}
	m.restoreSession()
	if m.currentSort().ID != helpers.FieldDirMtime {
		t.Fatalf("sort=%q", m.currentSort().ID)
	}
}
