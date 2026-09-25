package main

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const listViewTopOffset = 1 // leading newline in model.View()

func (m *model) itemDelegateSize() (height, spacing int) {
	if m.tableEnabled() || m.layout == layoutCompact {
		return 1, 0
	}
	return 2, 1
}

type listViewRegions struct {
	titleLines  int
	headerLines int // table column header (table mode only)
	itemsStart  int
	itemsEnd    int
	helpStart   int
	helpLines   int
}

func (m *model) listViewRegions() listViewRegions {
	l := m.list
	regions := listViewRegions{}

	if l.ShowTitle() || (l.ShowFilter() && l.FilteringEnabled()) {
		// The title bar style carries bottom padding, so it spans more than
		// the single text line.
		regions.titleLines = lipgloss.Height(l.Styles.TitleBar.Render(" "))
	}

	helpLines := 0
	if l.ShowHelp() {
		helpLines = lipgloss.Height(l.Styles.HelpStyle.Render(
			styledShortHelpView(l.Help, l.ShortHelp()),
		))
	}
	regions.helpLines = helpLines

	pagLines := 0
	if block := m.listPaginationBlock(); block != "" {
		pagLines = lipgloss.Height(block)
	}

	regions.headerLines = m.tableHeaderLines()
	total := lipgloss.Height(l.View()) + regions.headerLines
	regions.itemsStart = regions.titleLines + regions.headerLines
	regions.itemsEnd = total - pagLines - helpLines
	if regions.itemsEnd < regions.itemsStart {
		regions.itemsEnd = regions.itemsStart
	}

	regions.helpStart = total - helpLines
	return regions
}

func (m *model) visibleHelpBindingAtX(x int) (key.Binding, bool) {
	l := m.list
	bindings := l.ShortHelp()
	help := l.Help
	separator := help.Styles.ShortSeparator.Inline(true).Render(help.ShortSeparator)

	totalWidth := 0
	for _, kb := range bindings {
		if !kb.Enabled() {
			continue
		}

		sep := ""
		if totalWidth > 0 {
			sep = separator
		}
		item := sep + styledShortHelpItem(kb, help.Styles)
		w := lipgloss.Width(item)

		if help.Width > 0 && totalWidth+w > help.Width {
			tail := " " + help.Styles.Ellipsis.Inline(true).Render(help.Ellipsis)
			if totalWidth+lipgloss.Width(tail) < help.Width {
				break
			}
		}

		start := totalWidth
		end := totalWidth + w
		totalWidth += w

		if x >= start && x < end {
			return kb, true
		}
	}
	return key.Binding{}, false
}

func keyMsgFromString(s string) (tea.KeyMsg, bool) {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}, true
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}, true
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}, true
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}, true
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}, true
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}, true
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}, true
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}, true
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}, true
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}, true
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}, true
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}, true
	case "f5":
		return tea.KeyMsg{Type: tea.KeyF5}, true
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}, true
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}, true
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}, true
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}, true
	case "ctrl+n":
		return tea.KeyMsg{Type: tea.KeyCtrlN}, true
	case "ctrl+z":
		return tea.KeyMsg{Type: tea.KeyCtrlZ}, true
	default:
		if len(s) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(s[0])}}, true
		}
	}
	return tea.KeyMsg{}, false
}

func (m *model) invokeHelpBinding(b key.Binding) tea.Cmd {
	for _, keyStr := range b.Keys() {
		msg, ok := keyMsgFromString(keyStr)
		if !ok {
			continue
		}
		if cmd, handled := m.handleAppKeys(msg); handled {
			return cmd
		}
		var listCmd tea.Cmd
		m.list, listCmd = m.list.Update(msg)
		m.refreshTitle()
		m.persistSession()
		return listCmd
	}
	return nil
}

func (m *model) handleListItemClick(index int) tea.Cmd {
	if index < 0 || index >= len(m.list.VisibleItems()) {
		return nil
	}
	m.list.Select(index)
	if m.bulk.isActive() {
		if item, ok := m.list.SelectedItem().(projectItem); ok {
			return m.toggleBulkItem(item.path)
		}
		return nil
	}
	if item, ok := m.list.SelectedItem().(projectItem); ok {
		m.openDetails(item.path)
	}
	return nil
}

func mouseWheelDelta(msg tea.MouseMsg) (int, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return -1, true
	case tea.MouseButtonWheelDown:
		return 1, true
	default:
		return 0, false
	}
}

func (m *model) scrollListBy(delta int, l *list.Model) {
	if delta == 0 || l == nil {
		return
	}
	if delta < 0 {
		for i := 0; i < -delta; i++ {
			l.CursorUp()
		}
		return
	}
	for i := 0; i < delta; i++ {
		l.CursorDown()
	}
}

func (m *model) scrollHelpBy(delta int) {
	if delta == 0 || len(m.help.lines) == 0 {
		return
	}
	m.help.cursor += delta
	if m.help.cursor < 0 {
		m.help.cursor = 0
	}
	if m.help.cursor >= len(m.help.lines) {
		m.help.cursor = len(m.help.lines) - 1
	}
}

func (m *model) scrollLayoutMenuBy(delta int) {
	n := len(m.layoutMenu.rows)
	if delta == 0 || n == 0 {
		return
	}
	m.layoutMenu.cursor += delta
	if m.layoutMenu.cursor < 0 {
		m.layoutMenu.cursor = 0
	}
	if m.layoutMenu.cursor >= n {
		m.layoutMenu.cursor = n - 1
	}
}

func (m *model) scrollWriteMenuBy(delta int) {
	if delta == 0 || len(writeFormatEntries) == 0 {
		return
	}
	m.writeMenu.cursor += delta
	if m.writeMenu.cursor < 0 {
		m.writeMenu.cursor = 0
	}
	if m.writeMenu.cursor >= len(writeFormatEntries) {
		m.writeMenu.cursor = len(writeFormatEntries) - 1
	}
}

func (m *model) scrollWriteClipboardBy(delta int) {
	if delta == 0 || len(writeClipboardEntries) == 0 {
		return
	}
	m.writeMenu.clipboardCursor += delta
	if m.writeMenu.clipboardCursor < 0 {
		m.writeMenu.clipboardCursor = 0
	}
	if m.writeMenu.clipboardCursor >= len(writeClipboardEntries) {
		m.writeMenu.clipboardCursor = len(writeClipboardEntries) - 1
	}
}

func (m *model) scrollTagSuggestionsBy(delta int) {
	if delta == 0 {
		return
	}
	m.moveTagSuggestion(delta)
}

func (m *model) scrollTagOrderBy(delta int) {
	n := len(m.tagOrder.order)
	if delta == 0 || n == 0 {
		return
	}
	m.tagOrder.cursor += delta
	if m.tagOrder.cursor < 0 {
		m.tagOrder.cursor = 0
	}
	if m.tagOrder.cursor >= n {
		m.tagOrder.cursor = n - 1
	}
}

func (m *model) handleMouseWheel(msg tea.MouseMsg) (tea.Cmd, bool) {
	delta, ok := mouseWheelDelta(msg)
	if !ok {
		return nil, false
	}

	switch {
	case m.help.open:
		m.scrollHelpBy(delta)
		return nil, true
	case m.advFilter.open:
		m.scrollListBy(delta, &m.advFilter.list)
		return nil, true
	case m.layoutMenu.open:
		m.scrollLayoutMenuBy(delta)
		return nil, true
	case m.writeMenu.open && m.writeMenu.stage == writeStageFormat:
		m.scrollWriteMenuBy(delta)
		return nil, true
	case m.writeMenu.open && m.writeMenu.stage == writeStageClipboard:
		m.scrollWriteClipboardBy(delta)
		return nil, true
	case m.sortMenu.open:
		m.scrollSortMenuBy(delta)
		return nil, true
	case m.tagOrder.open:
		m.scrollTagOrderBy(delta)
		return nil, true
	case m.annotate.open && m.annotate.stage == annotateStageKey:
		m.scrollListBy(delta, &m.annotate.keyList)
		return nil, true
	case m.tagging:
		m.scrollTagSuggestionsBy(delta)
		return nil, true
	case m.details.open:
		if delta < 0 {
			m.detailsStep(-1)
		} else {
			m.detailsStep(+1)
		}
		return nil, true
	default:
		m.scrollListBy(delta, &m.list)
		m.refreshTitle()
		m.persistSession()
		return nil, true
	}
}

func (m *model) handleMouse(msg tea.MouseMsg) (model, tea.Cmd, bool) {
	if msg.Button != tea.MouseButtonLeft {
		return *m, nil, false
	}
	if msg.Action != tea.MouseActionPress && msg.Action != tea.MouseActionRelease {
		return *m, nil, false
	}

	listY := msg.Y - listViewTopOffset
	if listY < 0 {
		return *m, nil, false
	}

	regions := m.listViewRegions()

	if regions.helpLines > 0 && listY >= regions.helpStart && listY < regions.helpStart+regions.helpLines {
		if kb, ok := m.visibleHelpBindingAtX(msg.X); ok {
			return *m, m.invokeHelpBinding(kb), true
		}
		return *m, nil, true
	}

	if m.list.FilterState() == list.Filtering {
		return *m, nil, false
	}

	if regions.headerLines > 0 && listY >= regions.titleLines && listY < regions.titleLines+regions.headerLines {
		if msg.Action != tea.MouseActionPress {
			return *m, nil, true
		}
		return *m, m.handleTableHeaderClick(msg.X), true
	}

	if listY < regions.itemsStart || listY >= regions.itemsEnd {
		return *m, nil, false
	}

	delegateH, spacing := m.itemDelegateSize()
	block := delegateH + spacing
	if block <= 0 {
		return *m, nil, false
	}

	offset := listY - regions.itemsStart
	row := offset / block
	within := offset % block
	if within >= delegateH {
		return *m, nil, false
	}
	if row >= m.list.Paginator.PerPage {
		return *m, nil, false
	}

	return *m, m.handleListItemClick(row + m.list.Paginator.Page*m.list.Paginator.PerPage), true
}
