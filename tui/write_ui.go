package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type writeStage int

const (
	writeStageFormat writeStage = iota
	writeStagePath
	writeStageClipboard
)

var writeClipboardEntries = []struct {
	title string
	desc  string
	yes   bool
}{
	{title: "No", desc: "do not copy the file path", yes: false},
	{title: "Yes", desc: "copy the file path to the clipboard", yes: true},
}

type writeUI struct {
	open            bool
	stage           writeStage
	cursor          int
	format          writeFormat
	pathInput       textinput.Model
	clipboardCursor int
	defaultPath     string
	writtenPath     string
	deleteOnExit    bool
}

func newWriteUI() writeUI {
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 64
	return writeUI{pathInput: ti}
}

var (
	writeUp    = key.NewBinding(key.WithKeys("k", "up"))
	writeDown  = key.NewBinding(key.WithKeys("j", "down"))
	writePick  = key.NewBinding(key.WithKeys("enter"))
	writeClose = key.NewBinding(key.WithKeys("esc", "q", "w"))
)

func (m *model) openWriteMenu() {
	u := &m.writeMenu
	u.open = true
	u.stage = writeStageFormat
	u.cursor = 0
	u.format = writeFormatMarkdown
	u.defaultPath = ""
	u.writtenPath = ""
	u.deleteOnExit = false
	u.clipboardCursor = 0
	u.pathInput.SetValue("")
	u.pathInput.Blur()
}

func (u *writeUI) cleanup() {
	if u.deleteOnExit && u.writtenPath != "" {
		_ = osRemove(u.writtenPath)
	}
}

func (m *model) beginWritePathStage() tea.Cmd {
	u := &m.writeMenu
	entry := writeFormatEntries[u.cursor]
	u.format = entry.format
	path, err := defaultWritePath(u.format)
	if err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("write error: %v", err))
	}
	u.defaultPath = path
	u.stage = writeStagePath
	u.pathInput.SetValue(path)
	u.pathInput.Focus()
	return textinput.Blink
}

func (m *model) beginWriteClipboardStage() tea.Cmd {
	u := &m.writeMenu
	u.stage = writeStageClipboard
	u.clipboardCursor = 0
	u.pathInput.Blur()
	return nil
}

func (m *model) commitWrite(copyToClipboard bool) tea.Cmd {
	u := &m.writeMenu
	path := strings.TrimSpace(u.pathInput.Value())
	u.pathInput.Blur()
	u.open = false
	u.stage = writeStageFormat

	if err := m.writeCurrentView(u.format, path); err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("write error: %v", err))
	}

	u.writtenPath = path
	if u.defaultPath != "" && filepath.Clean(path) == filepath.Clean(u.defaultPath) {
		u.deleteOnExit = true
	}

	label := writeFormatEntries[u.format].title
	count := len(m.exportProjects())
	status := fmt.Sprintf("wrote %d projects · %s · %s", count, label, path)
	if copyToClipboard {
		return m.copyText(path, status+" · copied")
	}
	return m.list.NewStatusMessage(status)
}

func (m model) updateWriteMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		u := &m.writeMenu
		switch u.stage {
		case writeStagePath:
			return m.updateWritePath(msg)
		case writeStageClipboard:
			return m.updateWriteClipboard(msg)
		}
		switch {
		case key.Matches(msg, writeClose):
			u.open = false
			return m, nil
		case key.Matches(msg, writeUp):
			if u.cursor > 0 {
				u.cursor--
			}
		case key.Matches(msg, writeDown):
			if u.cursor < len(writeFormatEntries)-1 {
				u.cursor++
			}
		case key.Matches(msg, writePick):
			return m, m.beginWritePathStage()
		}
	}
	return m, nil
}

func (m model) updateWritePath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := &m.writeMenu
	switch msg.Type {
	case tea.KeyEsc:
		u.stage = writeStageFormat
		u.cursor = int(u.format)
		u.pathInput.Blur()
		return m, nil
	case tea.KeyEnter:
		return m, m.beginWriteClipboardStage()
	}
	var cmd tea.Cmd
	u.pathInput, cmd = u.pathInput.Update(msg)
	return m, cmd
}

func (m model) updateWriteClipboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := &m.writeMenu
	switch {
	case key.Matches(msg, writeClose):
		u.open = false
		return m, nil
	case key.Matches(msg, writeUp):
		if u.clipboardCursor > 0 {
			u.clipboardCursor--
		}
	case key.Matches(msg, writeDown):
		if u.clipboardCursor < len(writeClipboardEntries)-1 {
			u.clipboardCursor++
		}
	case msg.Type == tea.KeyEsc:
		u.stage = writeStagePath
		u.pathInput.Focus()
		return m, textinput.Blink
	case key.Matches(msg, writePick):
		copyPath := writeClipboardEntries[u.clipboardCursor].yes
		return m, m.commitWrite(copyPath)
	}
	return m, nil
}

func (m model) writeMenuView() string {
	u := m.writeMenu
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Write"))
	b.WriteString("\n\n")

	if u.stage == writeStagePath {
		entry := writeFormatEntries[u.format]
		b.WriteString(dim.Render(fmt.Sprintf("Format · %s · %d projects in current view", entry.title, len(m.exportProjects()))))
		b.WriteString("\n\n")
		b.WriteString(dim.Render("Write to file:"))
		b.WriteString("\n\n")
		b.WriteString(u.pathInput.View())
		b.WriteString("\n\n")
		b.WriteString(dim.Render("enter continue · esc back · w close"))
		return b.String()
	}

	if u.stage == writeStageClipboard {
		entry := writeFormatEntries[u.format]
		path := strings.TrimSpace(u.pathInput.Value())
		b.WriteString(dim.Render(fmt.Sprintf("Format · %s · %d projects", entry.title, len(m.exportProjects()))))
		b.WriteString("\n\n")
		b.WriteString(dim.Render("File"))
		b.WriteString("\n")
		b.WriteString("  " + path + "\n\n")
		b.WriteString(dim.Render("Copy path to clipboard?"))
		b.WriteString("\n")
		for i, choice := range writeClipboardEntries {
			line := choice.title
			if i == u.clipboardCursor {
				b.WriteString(cur.Render("> "+line) + "\n")
				b.WriteString("    " + dim.Render(choice.desc) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
				b.WriteString("    " + dim.Render(choice.desc) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(dim.Render("j/k move · enter write · esc back · w close"))
		return b.String()
	}

	b.WriteString(dim.Render("Export the current view (filters, sort, quick filter) with all fields"))
	b.WriteString("\n\n")
	for i, entry := range writeFormatEntries {
		line := entry.title
		if i == u.cursor {
			b.WriteString(cur.Render("> "+line) + "\n")
			b.WriteString("    " + dim.Render(entry.desc) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
			b.WriteString("    " + dim.Render(entry.desc) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(dim.Render("j/k move · enter choose format · esc close"))
	return b.String()
}

// osRemove exists so tests can stub file deletion.
var osRemove = os.Remove
