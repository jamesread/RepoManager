package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMenuKind int

const (
	viewItemSave viewMenuKind = iota
	viewItemLoad
)

type viewMenuEntry struct {
	kind       viewMenuKind
	id         string
	title      string
	desc       string
	predefined bool
}

type viewUI struct {
	open      bool
	cursor    int
	entries   []viewMenuEntry
	naming    bool
	nameInput textinput.Model
}

func newViewUI() viewUI {
	ti := textinput.New()
	ti.Placeholder = "view name"
	ti.CharLimit = 48
	ti.Width = 32
	return viewUI{nameInput: ti}
}

var (
	viewUp    = key.NewBinding(key.WithKeys("k", "up"))
	viewDown  = key.NewBinding(key.WithKeys("j", "down"))
	viewPick  = key.NewBinding(key.WithKeys("enter"))
	viewClose = key.NewBinding(key.WithKeys("esc", "q", "v"))
)

func (m *model) openViewMenu() {
	m.viewMenu.open = true
	m.viewMenu.naming = false
	m.viewMenu.cursor = 0
	m.viewMenu.nameInput.Blur()
	m.viewMenu.entries = m.buildViewMenuEntries()
}

func (m *model) buildViewMenuEntries() []viewMenuEntry {
	entries := []viewMenuEntry{{
		kind:  viewItemSave,
		title: "Save current view as…",
		desc:  "layout, sort, and filters",
	}}
	for _, p := range predefinedViews {
		entries = append(entries, viewMenuEntry{
			kind:       viewItemLoad,
			id:         p.ID,
			title:      p.Name,
			desc:       p.Desc,
			predefined: true,
		})
	}
	for _, sv := range m.store.ViewsFor(m.root) {
		entries = append(entries, viewMenuEntry{
			kind:  viewItemLoad,
			id:    savedViewID(sv.Name),
			title: sv.Name,
			desc:  "saved view",
		})
	}
	return entries
}

func (m *model) beginViewSave() {
	m.viewMenu.naming = true
	m.viewMenu.nameInput.SetValue("")
	m.viewMenu.nameInput.Focus()
}

func (m model) updateViewMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		if m.viewMenu.naming {
			return m.updateViewNaming(msg)
		}
		u := &m.viewMenu
		n := len(u.entries)
		switch {
		case key.Matches(msg, viewClose):
			u.open = false
			return m, nil
		case n == 0:
			return m, nil
		case key.Matches(msg, viewUp):
			if u.cursor > 0 {
				u.cursor--
			}
		case key.Matches(msg, viewDown):
			if u.cursor < n-1 {
				u.cursor++
			}
		case key.Matches(msg, viewPick):
			entry := u.entries[u.cursor]
			switch entry.kind {
			case viewItemSave:
				m.beginViewSave()
				return m, textinput.Blink
			case viewItemLoad:
				u.open = false
				if err := m.loadViewByID(entry.id); err != nil {
					return m, m.list.NewStatusMessage(fmt.Sprintf("view error: %v", err))
				}
				label := m.activeViewTitle()
				if label == "" {
					label = entry.title
				}
				return m, m.list.NewStatusMessage("view · " + label)
			}
		}
	}
	return m, nil
}

func (m model) updateViewNaming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := &m.viewMenu
	switch msg.Type {
	case tea.KeyEsc:
		u.naming = false
		u.nameInput.Blur()
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(u.nameInput.Value())
		u.naming = false
		u.nameInput.Blur()
		u.open = false
		if err := m.saveCurrentView(name); err != nil {
			return m, m.list.NewStatusMessage(fmt.Sprintf("view error: %v", err))
		}
		m.publishList()
		m.refreshTitle()
		return m, m.list.NewStatusMessage("saved view · " + name)
	}
	var cmd tea.Cmd
	u.nameInput, cmd = u.nameInput.Update(msg)
	return m, cmd
}

func (m model) viewMenuView() string {
	u := m.viewMenu
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Views"))
	b.WriteString("\n\n")

	if u.naming {
		b.WriteString(dim.Render("Save current layout, sort, and filters as:"))
		b.WriteString("\n\n")
		b.WriteString(u.nameInput.View())
		b.WriteString("\n\n")
		b.WriteString(dim.Render("enter save · esc cancel"))
		return b.String()
	}

	active := m.activeViewTitle()
	if active != "" {
		b.WriteString(dim.Render("Active · " + active))
		b.WriteString("\n\n")
	}

	showSection := func(title string) {
		b.WriteString(dim.Render(title))
		b.WriteString("\n")
	}

	inPredefined := false
	inSaved := false
	for i, entry := range u.entries {
		if entry.kind == viewItemSave {
			line := entry.title
			if i == u.cursor {
				b.WriteString(cur.Render("> "+line) + "\n")
				b.WriteString("    " + dim.Render(entry.desc) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
				b.WriteString("    " + dim.Render(entry.desc) + "\n")
			}
			continue
		}
		if entry.predefined && !inPredefined {
			if i > 0 {
				b.WriteString("\n")
			}
			showSection("Predefined")
			inPredefined = true
		}
		if !entry.predefined && entry.kind == viewItemLoad && !inSaved {
			if inPredefined {
				b.WriteString("\n")
			}
			showSection("Saved")
			inSaved = true
		}

		marker := "  "
		line := entry.title
		if entry.predefined {
			line += " · preset"
		}
		if entry.id == m.activeViewID {
			line += " · active"
		}
		if i == u.cursor {
			marker = cur.Render("> ")
			b.WriteString(marker + cur.Render(line) + "\n")
			b.WriteString("    " + dim.Render(entry.desc) + "\n")
		} else {
			b.WriteString(marker + line + "\n")
			b.WriteString("    " + dim.Render(entry.desc) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dim.Render("j/k move · enter load/save · esc close"))
	return b.String()
}
