package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func testMouseModel(t *testing.T, items []list.Item, layout layoutMode) *model {
	t.Helper()

	st := newTestStore(t)
	bulk := newBulkSelectState()
	delegate := newItemDelegate(st, nil, layout, newRefreshState(), t.TempDir(), bulk)
	l := list.New(items, delegate, 80, 24)
	l.Title = "RepoManager"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "help")),
		}
	}

	return &model{
		list:   l,
		store:  st,
		layout: layout,
		width:  80,
		height: 24,
		bulk:   bulk,
	}
}

func TestVisibleHelpBindingAtX(t *testing.T) {
	m := testMouseModel(t, nil, layoutComfy)
	kb, ok := m.visibleHelpBindingAtX(0)
	if !ok {
		t.Fatal("expected binding at x=0")
	}
	if kb.Help().Desc != "up" && kb.Help().Desc != "move up" {
		// first binding is cursor up from list defaults
		if kb.Help().Key == "" {
			t.Fatalf("unexpected binding: %+v", kb.Help())
		}
	}
}

func TestListItemClickIndex(t *testing.T) {
	items := []list.Item{
		projectItem{path: "/a", rel: "a"},
		projectItem{path: "/b", rel: "b"},
		projectItem{path: "/c", rel: "c"},
	}
	m := testMouseModel(t, items, layoutComfy)
	m.list.SetItems(items)

	regions := m.listViewRegions()
	// Default title bar = title line + one padding line.
	if regions.titleLines != 2 {
		t.Fatalf("titleLines = %d, want 2", regions.titleLines)
	}
	// The rendered view must place the first item exactly at itemsStart.
	lines := strings.Split(ansi.Strip(m.mainListView()), "\n")
	if !strings.Contains(lines[regions.itemsStart], "a") {
		t.Fatalf("first item not at itemsStart=%d: %q", regions.itemsStart, lines[regions.itemsStart])
	}

	// First item row starts right after the title.
	listY := regions.itemsStart
	index := listY - regions.itemsStart
	delegateH, spacing := m.itemDelegateSize()
	block := delegateH + spacing
	row := index / block
	want := 0
	if row != want {
		t.Fatalf("row = %d, want %d", row, want)
	}

	m.handleListItemClick(row)
	if got, ok := m.list.SelectedItem().(projectItem); !ok || got.path != "/a" {
		t.Fatalf("selected = %v, want /a", m.list.SelectedItem())
	}
	if !m.details.open {
		t.Fatal("expected details view to open")
	}
}

func TestListViewRegionsCompact(t *testing.T) {
	items := []list.Item{projectItem{path: "/a", rel: "a"}}
	m := testMouseModel(t, items, layoutCompact)
	m.list.SetItems(items)

	h, spacing := m.itemDelegateSize()
	if h != 1 || spacing != 0 {
		t.Fatalf("delegate size = (%d,%d), want (1,0)", h, spacing)
	}

	regions := m.listViewRegions()
	if regions.itemsEnd <= regions.itemsStart {
		t.Fatalf("items region empty: %+v", regions)
	}
}

func TestListWheelScroll(t *testing.T) {
	st := newTestStore(t)
	items := []list.Item{
		projectItem{path: "/a", rel: "a"},
		projectItem{path: "/b", rel: "b"},
		projectItem{path: "/c", rel: "c"},
	}
	m := testMouseModel(t, items, layoutComfy)
	m.store = st
	m.list.SetItems(items)

	_, handled := m.handleMouseWheel(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if !handled {
		t.Fatal("expected wheel down to be handled")
	}
	if m.list.Index() != 1 {
		t.Fatalf("index=%d, want 1", m.list.Index())
	}

	_, handled = m.handleMouseWheel(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if !handled {
		t.Fatal("expected wheel up to be handled")
	}
	if m.list.Index() != 0 {
		t.Fatalf("index=%d, want 0", m.list.Index())
	}
}
