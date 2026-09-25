package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/helpers"
)

// sortUI is the menu opened with `S`. Selected fields are listed first in
// priority order; space toggles a field, J/K move a selected field's
// priority. Changes apply immediately.
type sortUI struct {
	open   bool
	cursor int
}

type sortMenuEntry struct {
	field    helpers.Field
	selected bool
	desc     bool
	rank     int // 1-based priority when selected
}

var (
	sortMenuUp        = key.NewBinding(key.WithKeys("k", "up"))
	sortMenuDown      = key.NewBinding(key.WithKeys("j", "down"))
	sortMenuToggle    = key.NewBinding(key.WithKeys(" "))
	sortMenuDirection = key.NewBinding(key.WithKeys("d", "left", "right", "tab"))
	sortMenuMvUp      = key.NewBinding(key.WithKeys("K", "shift+up"))
	sortMenuMvDn      = key.NewBinding(key.WithKeys("J", "shift+down"))
	sortMenuTop       = key.NewBinding(key.WithKeys("g", "home"))
	sortMenuEnd       = key.NewBinding(key.WithKeys("G", "end"))
	sortMenuClose     = key.NewBinding(key.WithKeys("esc", "q", "S", "enter"))
)

func (m *model) sortMenuEntries() []sortMenuEntry {
	selected := m.resolvedSortKeys()
	chosen := make(map[string]bool, len(selected))
	entries := make([]sortMenuEntry, 0, len(selected))
	for i, k := range selected {
		chosen[k.Field.ID] = true
		entries = append(entries, sortMenuEntry{field: k.Field, selected: true, desc: k.Desc, rank: i + 1})
	}
	for _, f := range m.sortableFields() {
		if chosen[f.ID] {
			continue
		}
		entries = append(entries, sortMenuEntry{field: f})
	}
	return entries
}

func (m *model) openSortMenu() {
	m.sortMenu.open = true
	m.sortMenu.cursor = 0
}

func (m *model) sortMenuCursorTo(id string) {
	for i, e := range m.sortMenuEntries() {
		if e.field.ID == id {
			m.sortMenu.cursor = i
			return
		}
	}
}

func (m *model) applySortChange() tea.Cmd {
	m.publishList()
	m.refreshTitle()
	m.persistSession()
	return m.list.NewStatusMessage("sort · " + m.sortTitleLabel())
}

func (m *model) scrollSortMenuBy(delta int) {
	n := len(m.sortMenuEntries())
	if delta == 0 || n == 0 {
		return
	}
	m.sortMenu.cursor += delta
	if m.sortMenu.cursor < 0 {
		m.sortMenu.cursor = 0
	}
	if m.sortMenu.cursor >= n {
		m.sortMenu.cursor = n - 1
	}
}

func (m model) updateSortMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		u := &m.sortMenu
		entries := m.sortMenuEntries()
		n := len(entries)
		switch {
		case key.Matches(msg, sortMenuClose):
			u.open = false
			return m, nil
		case n == 0:
			return m, nil
		case key.Matches(msg, sortMenuUp):
			if u.cursor > 0 {
				u.cursor--
			}
		case key.Matches(msg, sortMenuDown):
			if u.cursor < n-1 {
				u.cursor++
			}
		case key.Matches(msg, sortMenuTop):
			u.cursor = 0
		case key.Matches(msg, sortMenuEnd):
			u.cursor = n - 1
		case key.Matches(msg, sortMenuToggle):
			if u.cursor >= n {
				u.cursor = n - 1
			}
			e := entries[u.cursor]
			m.toggleSortField(e.field.ID)
			m.sortMenuCursorTo(e.field.ID)
			return m, m.applySortChange()
		case key.Matches(msg, sortMenuDirection):
			if u.cursor >= n {
				u.cursor = n - 1
			}
			e := entries[u.cursor]
			if !e.selected {
				// Selecting via the direction key is a convenience: add it,
				// then flip from the field's default.
				m.toggleSortField(e.field.ID)
			}
			m.toggleSortDirection(e.field.ID)
			m.sortMenuCursorTo(e.field.ID)
			return m, m.applySortChange()
		case key.Matches(msg, sortMenuMvUp), key.Matches(msg, sortMenuMvDn):
			if u.cursor >= n {
				u.cursor = n - 1
			}
			e := entries[u.cursor]
			if !e.selected {
				return m, nil
			}
			delta := -1
			if key.Matches(msg, sortMenuMvDn) {
				delta = +1
			}
			if m.moveSortField(e.field.ID, delta) {
				m.sortMenuCursorTo(e.field.ID)
				return m, m.applySortChange()
			}
		}
	}
	return m, nil
}

func (m model) sortMenuView() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	entries := m.sortMenuEntries()
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Sort"))
	b.WriteString("\n")
	b.WriteString(dim.Render("selected fields sort first, in priority order · name breaks ties"))
	b.WriteString("\n")
	b.WriteString(dim.Render(sortGlyphAsc + " ascending · " + sortGlyphDesc + " descending"))
	b.WriteString("\n\n")

	avail := m.height - 8
	if avail < 3 {
		avail = 3
	}
	start := 0
	if m.sortMenu.cursor >= avail {
		start = m.sortMenu.cursor - avail + 1
	}
	end := start + avail
	if end > len(entries) {
		end = len(entries)
	}

	for i := start; i < end; i++ {
		e := entries[i]
		rank := "   "
		direction := ""
		if e.selected {
			rank = fmt.Sprintf("%2d.", e.rank)
			k := sortKey{Field: e.field, Desc: e.desc}
			direction = fmt.Sprintf("  %s %s", k.glyph(), k.directionLabel())
		}
		line := fmt.Sprintf("%s %s %s", tickMark(e.selected), rank, e.field.Label)
		if i == m.sortMenu.cursor {
			b.WriteString(cur.Render("> "+line) + dim.Render(direction) + "\n")
		} else if e.selected {
			b.WriteString("  " + line + dim.Render(direction) + "\n")
		} else {
			b.WriteString("  " + dim.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dim.Render("j/k move · space select · d direction · J/K priority · esc close"))
	return b.String()
}
