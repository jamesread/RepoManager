package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/tui/widgets"
)

// layoutMode selects how each project row is drawn in the main list.
type layoutMode string

const (
	layoutComfy   layoutMode = "comfy"
	layoutCompact layoutMode = "compact"
)

func parseLayoutMode(s string) layoutMode {
	switch layoutMode(strings.ToLower(strings.TrimSpace(s))) {
	case layoutCompact:
		return layoutCompact
	default:
		return layoutComfy
	}
}

// layoutMenuKind identifies which setting a layout menu row controls.
type layoutMenuKind int

const (
	layoutItemCompact layoutMenuKind = iota
	layoutItemMode
	layoutItemPathStyle
	layoutItemTilde
)

// Option IDs shared by the radio groups and their setters.
const (
	optOn  = "on"
	optOff = "off"

	layoutModeTable = "table"
	layoutModeList  = "list"

	pathStyleRelative = "relative"
	pathStyleAbsolute = "absolute"
)

// layoutMenuRow pairs a widgets.RadioGroup with the setting it drives.
type layoutMenuRow struct {
	kind  layoutMenuKind
	radio widgets.RadioGroup
}

// layoutUI is the menu opened with `l`. Every row is a widgets.RadioGroup; the rows
// are rebuilt from the current settings each time the menu opens.
type layoutUI struct {
	open   bool
	cursor int
	rows   []layoutMenuRow
}

var (
	layoutUp    = key.NewBinding(key.WithKeys("k", "up"))
	layoutDown  = key.NewBinding(key.WithKeys("j", "down"))
	layoutClose = key.NewBinding(key.WithKeys("esc", "q", "l"))
)

func onOff(on bool) string {
	if on {
		return optOn
	}
	return optOff
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// layoutMenuRows builds the menu from the model's current settings.
func (m *model) layoutMenuRows() []layoutMenuRow {
	compact := widgets.NewRadioGroup("Compact layout",
		widgets.RadioOption{ID: optOn, Label: "On", Desc: "one line per project (name and tags only); C toggles"},
		widgets.RadioOption{ID: optOff, Label: "Off", Desc: "two lines per project: name, then path and details; C toggles"},
	)
	compact.Select(onOff(m.layout == layoutCompact))

	mode := widgets.NewRadioGroup("Layout mode",
		widgets.RadioOption{ID: layoutModeList, Label: "List", Desc: "one block per project, tags on the right"},
		widgets.RadioOption{ID: layoutModeTable, Label: "Table", Desc: "values aligned in columns under a header (T toggles; click a header to sort)"},
	)
	mode.Select(pick(m.tableEnabled(), layoutModeTable, layoutModeList))

	pathStyle := widgets.NewRadioGroup("Path style",
		widgets.RadioOption{ID: pathStyleRelative, Label: "Relative", Desc: "show paths relative to the scan directory"},
		widgets.RadioOption{ID: pathStyleAbsolute, Label: "Absolute", Desc: "show full filesystem paths"},
	)
	pathStyle.Select(pick(m.store.PathRelative(), pathStyleRelative, pathStyleAbsolute))

	tilde := widgets.NewRadioGroup("Path display: ~ for $HOME",
		widgets.RadioOption{ID: optOn, Label: "On", Desc: "show ~/… instead of $HOME/…"},
		widgets.RadioOption{ID: optOff, Label: "Off", Desc: "show the full home directory path"},
	)
	tilde.Select(onOff(m.store.PathTilde()))

	return []layoutMenuRow{
		{layoutItemCompact, compact},
		{layoutItemMode, mode},
		{layoutItemPathStyle, pathStyle},
		{layoutItemTilde, tilde},
	}
}

func (m *model) openLayoutMenu() {
	m.layoutMenu.open = true
	m.layoutMenu.cursor = 0
	m.layoutMenu.rows = m.layoutMenuRows()
}

// layoutMenuContentWidth is the width available to a menu row, leaving a
// small right margin.
func (m *model) layoutMenuContentWidth() int {
	if m.width <= 0 {
		return 0 // unknown: let controls choose their natural shape
	}
	w := m.width - 2
	if w < 1 {
		w = 1
	}
	return w
}

// applyLayoutChoice routes a radio selection to the matching setter.
func (m *model) applyLayoutChoice(kind layoutMenuKind, id string) tea.Cmd {
	switch kind {
	case layoutItemCompact:
		return m.setCompactLayout(id == optOn)
	case layoutItemMode:
		return m.setTableMode(id == layoutModeTable)
	case layoutItemPathStyle:
		return m.setPathStyle(id == pathStyleRelative)
	case layoutItemTilde:
		return m.setPathTilde(id == optOn)
	}
	return nil
}

func (m *model) applyLayout(mode layoutMode) {
	m.layout = mode
	d := newItemDelegate(m.store, func() []helpers.Field { return m.helperFields() }, mode, m.refresh, m.root, m.bulk)
	d.table = m.table
	m.list.SetDelegate(d)
	m.recomputeTableColumns()
}

// setCompactLayout switches between the one- and two-line list rows.
func (m *model) setCompactLayout(on bool) tea.Cmd {
	mode := layoutComfy
	if on {
		mode = layoutCompact
	}
	if mode == m.layout {
		return nil
	}
	m.applyLayout(mode)
	if err := m.engSend(engine.SetLayoutRequest{Layout: string(mode)}); err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("layout error: %v", err))
	}
	m.refreshTitle()
	m.publishList()
	if on {
		return m.list.NewStatusMessage("layout · compact")
	}
	return m.list.NewStatusMessage("layout · comfy")
}

// toggleCompactLayout is the `C` quick shortcut. It applies in table mode
// too; the setting takes visible effect once table mode is left.
func (m *model) toggleCompactLayout() tea.Cmd {
	return m.setCompactLayout(m.layout != layoutCompact)
}

// setPathStyle persists relative/absolute paths and redraws.
func (m *model) setPathStyle(relative bool) tea.Cmd {
	if m.store.PathRelative() == relative {
		return nil
	}
	m.applyLayout(m.layout) // refresh delegate path formatting
	if err := m.engSend(engine.SetPathRelativeRequest{Relative: relative}); err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("layout error: %v", err))
	}
	m.refreshTitle()
	m.publishList()
	if relative {
		return m.list.NewStatusMessage("paths · relative")
	}
	return m.list.NewStatusMessage("paths · absolute")
}

// setPathTilde persists whether $HOME is shown as ~ and redraws.
func (m *model) setPathTilde(on bool) tea.Cmd {
	if m.store.PathTilde() == on {
		return nil
	}
	m.applyLayout(m.layout)
	if err := m.engSend(engine.SetPathTildeRequest{Tilde: on}); err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("layout error: %v", err))
	}
	m.refreshTitle()
	m.publishList()
	if on {
		return m.list.NewStatusMessage("paths · ~ for home")
	}
	return m.list.NewStatusMessage("paths · full home path")
}

func (m model) updateLayoutMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		u := &m.layoutMenu
		if key.Matches(msg, layoutClose) {
			u.open = false
			return m, nil
		}
		if len(u.rows) == 0 {
			return m, nil
		}
		if u.cursor >= len(u.rows) {
			u.cursor = len(u.rows) - 1
		}
		// The focused widgets.RadioGroup gets first refusal on keys so ←/→/tab and
		// space/enter cycle its options; j/k fall through to move the cursor.
		row := &u.rows[u.cursor]
		if consumed, changed := row.radio.Update(msg); consumed {
			if changed {
				return m, m.applyLayoutChoice(row.kind, row.radio.SelectedID())
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, layoutUp):
			if u.cursor > 0 {
				u.cursor--
			}
		case key.Matches(msg, layoutDown):
			if u.cursor < len(u.rows)-1 {
				u.cursor++
			}
		}
	}
	return m, nil
}

func tickMark(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (m model) layoutMenuView() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Layout"))
	b.WriteString("\n\n")
	b.WriteString(dim.Render("Display"))
	b.WriteString("\n")
	width := m.layoutMenuContentWidth()
	for i, row := range m.layoutMenu.rows {
		r := row.radio
		r.Focused = i == m.layoutMenu.cursor
		b.WriteString(r.View(width))
	}
	b.WriteString("\n")
	b.WriteString(dim.Render("j/k move · ←/→/space choose · esc close"))
	return b.String()
}
