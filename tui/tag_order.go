package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/store"
)

// tagOrderUI is a full-screen editor for the global tag importance ranking.
// Tags are listed most-important first; j/k move the cursor, J/K (or
// shift+arrows) move the selected tag, enter saves, esc cancels.
type tagOrderUI struct {
	open   bool
	order  []string
	cursor int
	dirty  bool
	width  int
	height int
}

func newTagOrderUI() tagOrderUI {
	return tagOrderUI{}
}

func (u *tagOrderUI) setSize(w, h int) {
	u.width = w
	u.height = h
}

var (
	tagOrderUp   = key.NewBinding(key.WithKeys("k", "up"))
	tagOrderDown = key.NewBinding(key.WithKeys("j", "down"))
	tagOrderMvUp = key.NewBinding(key.WithKeys("K", "shift+up"))
	tagOrderMvDn = key.NewBinding(key.WithKeys("J", "shift+down"))
	tagOrderTop  = key.NewBinding(key.WithKeys("g", "home"))
	tagOrderEnd  = key.NewBinding(key.WithKeys("G", "end"))
	tagOrderSave = key.NewBinding(key.WithKeys("enter"))
	tagOrderQuit = key.NewBinding(key.WithKeys("esc", "q"))
)

func (m *model) openTagOrder() {
	// Seed with the saved ranking, then append any tags seen on projects
	// (or in memory) that are not yet ranked, so every tag can be placed.
	known := m.store.AllKnownTags()
	for _, p := range m.all {
		known = mergeTags(known, p.tags)
	}
	m.tagOrder.order = store.SortTagsByOrder(known, m.store.TagOrder())
	m.tagOrder.cursor = 0
	m.tagOrder.dirty = false
	m.tagOrder.open = true
}

func (m model) updateTagOrder(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.resizeList(msg.Width-2, msg.Height-4)
		m.tagOrder.setSize(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		u := &m.tagOrder
		n := len(u.order)
		switch {
		case key.Matches(msg, tagOrderQuit):
			u.open = false
			return m, nil
		case key.Matches(msg, tagOrderSave):
			u.open = false
			if !u.dirty {
				return m, nil
			}
			if err := m.engSend(engine.SetTagOrderRequest{Order: u.order}); err != nil {
				return m, m.list.NewStatusMessage(fmt.Sprintf("tag order error: %v", err))
			}
			m.publishList()
			return m, m.list.NewStatusMessage("tag order saved")
		case n == 0:
			return m, nil
		case key.Matches(msg, tagOrderUp):
			if u.cursor > 0 {
				u.cursor--
			}
		case key.Matches(msg, tagOrderDown):
			if u.cursor < n-1 {
				u.cursor++
			}
		case key.Matches(msg, tagOrderTop):
			u.cursor = 0
		case key.Matches(msg, tagOrderEnd):
			u.cursor = n - 1
		case key.Matches(msg, tagOrderMvUp):
			if u.cursor > 0 {
				u.order = moveTag(u.order, u.cursor, -1)
				u.cursor--
				u.dirty = true
			}
		case key.Matches(msg, tagOrderMvDn):
			if u.cursor < n-1 {
				u.order = moveTag(u.order, u.cursor, +1)
				u.cursor++
				u.dirty = true
			}
		}
	}
	return m, nil
}

func (u tagOrderUI) View() string {
	if !u.open {
		return ""
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	var b strings.Builder
	b.WriteString("\n")
	title := "Tag importance"
	if u.dirty {
		title += " (unsaved)"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dim.Render(fmt.Sprintf("most important first · top %d shown per project", maxVisibleTags)))
	b.WriteString("\n\n")

	if len(u.order) == 0 {
		b.WriteString(dim.Render("no tags yet"))
		b.WriteString("\n")
	}

	// Simple viewport so long lists stay within the terminal height.
	avail := u.height - 6
	if avail < 3 {
		avail = 3
	}
	start := 0
	if u.cursor >= avail {
		start = u.cursor - avail + 1
	}
	end := start + avail
	if end > len(u.order) {
		end = len(u.order)
	}

	for i := start; i < end; i++ {
		tag := u.order[i]
		marker := "  "
		rank := dim.Render(fmt.Sprintf("%2d.", i+1))
		if i == u.cursor {
			marker = cur.Render("> ")
			rank = cur.Render(fmt.Sprintf("%2d.", i+1))
		}
		fmt.Fprintf(&b, "%s%s %s\n", marker, rank, tag)
	}

	b.WriteString("\n")
	b.WriteString(dim.Render("j/k move · J/K reorder · g/G ends · enter save · esc cancel"))
	return b.String()
}
