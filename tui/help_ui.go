package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	keys string
	desc string
}

type helpSection struct {
	title string
	items []helpEntry
}

type helpContext int

const (
	helpContextList helpContext = iota
	helpContextQuickFilter
	helpContextDetails
	helpContextAdvFilterFields
	helpContextAdvFilterValues
	helpContextAdvFilterMulti
	helpContextLayout
	helpContextView
	helpContextWrite
	helpContextWritePath
	helpContextWriteClipboard
	helpContextTagOrder
	helpContextAnnotateKey
	helpContextAnnotateNewKey
	helpContextAnnotateValue
	helpContextTagging
	helpContextBulkSelect
	helpContextSortMenu
)

type helpUI struct {
	open   bool
	cursor int
	width  int
	height int
	lines  []string
	title  string
}

func newHelpUI() helpUI {
	return helpUI{}
}

func (u *helpUI) setSize(w, h int) {
	u.width = w
	u.height = h
}

var (
	helpToggle = key.NewBinding(key.WithKeys("?"))
	helpUp     = key.NewBinding(key.WithKeys("k", "up"))
	helpDown   = key.NewBinding(key.WithKeys("j", "down"))
	helpTop    = key.NewBinding(key.WithKeys("g", "home"))
	helpEnd    = key.NewBinding(key.WithKeys("G", "end"))
	helpClose  = key.NewBinding(key.WithKeys("esc", "q", "h", "?"))
)

func (m *model) currentHelpContext() helpContext {
	if m.advFilter.open {
		if m.advFilter.stage == filterStageValues {
			if m.advFilter.multiSelect {
				return helpContextAdvFilterMulti
			}
			return helpContextAdvFilterValues
		}
		return helpContextAdvFilterFields
	}
	if m.layoutMenu.open {
		return helpContextLayout
	}
	if m.viewMenu.open {
		return helpContextView
	}
	if m.writeMenu.open {
		switch m.writeMenu.stage {
		case writeStagePath:
			return helpContextWritePath
		case writeStageClipboard:
			return helpContextWriteClipboard
		default:
			return helpContextWrite
		}
	}
	if m.sortMenu.open {
		return helpContextSortMenu
	}
	if m.tagOrder.open {
		return helpContextTagOrder
	}
	if m.annotate.open {
		switch m.annotate.stage {
		case annotateStageNewKey:
			return helpContextAnnotateNewKey
		case annotateStageValue:
			return helpContextAnnotateValue
		default:
			return helpContextAnnotateKey
		}
	}
	if m.tagging {
		return helpContextTagging
	}
	if m.bulk.isActive() {
		return helpContextBulkSelect
	}
	if m.details.open {
		return helpContextDetails
	}
	if m.list.FilterState() == list.Filtering {
		return helpContextQuickFilter
	}
	return helpContextList
}

func helpTitleForContext(ctx helpContext) string {
	switch ctx {
	case helpContextQuickFilter:
		return "Help · quick filter"
	case helpContextDetails:
		return "Help · project details"
	case helpContextAdvFilterFields:
		return "Help · advanced filter"
	case helpContextAdvFilterValues:
		return "Help · filter values"
	case helpContextAdvFilterMulti:
		return "Help · filter values (multi)"
	case helpContextLayout:
		return "Help · layout"
	case helpContextView:
		return "Help · views"
	case helpContextWrite:
		return "Help · write"
	case helpContextWritePath:
		return "Help · write path"
	case helpContextWriteClipboard:
		return "Help · write clipboard"
	case helpContextTagOrder:
		return "Help · tag order"
	case helpContextAnnotateKey:
		return "Help · annotate"
	case helpContextAnnotateNewKey:
		return "Help · new annotation key"
	case helpContextAnnotateValue:
		return "Help · annotation value"
	case helpContextTagging:
		return "Help · add tag"
	case helpContextBulkSelect:
		return "Help · bulk select"
	case helpContextSortMenu:
		return "Help · sort"
	default:
		return "Help · project list"
	}
}

func helpSectionsForContext(ctx helpContext) []helpSection {
	switch ctx {
	case helpContextQuickFilter:
		return helpSectionsQuickFilter
	case helpContextDetails:
		return helpSectionsDetails
	case helpContextAdvFilterFields:
		return helpSectionsAdvFilterFields
	case helpContextAdvFilterValues:
		return helpSectionsAdvFilterValues
	case helpContextAdvFilterMulti:
		return helpSectionsAdvFilterMulti
	case helpContextLayout:
		return helpSectionsLayout
	case helpContextView:
		return helpSectionsView
	case helpContextWrite:
		return helpSectionsWrite
	case helpContextWritePath:
		return helpSectionsWritePath
	case helpContextWriteClipboard:
		return helpSectionsWriteClipboard
	case helpContextTagOrder:
		return helpSectionsTagOrder
	case helpContextAnnotateKey:
		return helpSectionsAnnotateKey
	case helpContextAnnotateNewKey:
		return helpSectionsAnnotateNewKey
	case helpContextAnnotateValue:
		return helpSectionsAnnotateValue
	case helpContextTagging:
		return helpSectionsTagging
	case helpContextBulkSelect:
		return helpSectionsBulkSelect
	case helpContextSortMenu:
		return helpSectionsSortMenu
	default:
		return helpSectionsList
	}
}

var helpSectionsList = []helpSection{
	{
		title: "Navigation",
		items: []helpEntry{
			{"j / ↓", "move down"},
			{"k / ↑", "move up"},
			{"g / home", "first item"},
			{"G / end", "last item"},
			{"/", "quick filter"},
			{"enter", "project details"},
			{"ctrl+z", "suspend to shell background"},
			{"q / esc", "quit"},
			{"?", "this help"},
		},
	},
	{
		title: "Projects",
		items: []helpEntry{
			{"P", "copy absolute path to clipboard"},
			{"y", "yank project details as markdown"},
			{"w", "write current view to file"},
			{"F5 / ctrl+r", "refresh helpers for selection"},
		},
	},
	{
		title: "Tags & annotations",
		items: []helpEntry{
			{"t", "add tag"},
			{"O", "tag importance order"},
			{"a", "edit annotations"},
			{"b", "bulk select projects"},
		},
	},
	{
		title: "Filter & sort",
		items: []helpEntry{
			{"F / ctrl+f", "advanced filter"},
			{"s", "cycle primary sort field"},
			{"S", "sort menu (multi-field priority)"},
		},
	},
	{
		title: "Display",
		items: []helpEntry{
			{"l", "layout menu (compact, table, paths)"},
			{"C", "toggle compact layout"},
			{"T", "toggle table mode (aligned columns)"},
			{"v", "views (save/load layout, sort, filters)"},
			{"h", "this help"},
		},
	},
	{
		title: "Mouse",
		items: []helpEntry{
			{"left click item", "open project details"},
			{"left click column header", "sort by column; click again to flip direction (table mode)"},
			{"left click shortcuts", "run shortcut"},
		},
	},
}

var helpSectionsSortMenu = []helpSection{
	{
		title: "Sort",
		items: []helpEntry{
			{"j / k", "move cursor"},
			{"space", "select / deselect field"},
			{"d / ← / → / tab", "toggle ascending / descending"},
			{"J / shift+↓", "lower priority of selected field"},
			{"K / shift+↑", "raise priority of selected field"},
			{"g / G", "first / last field"},
			{"esc / q / S / enter", "close"},
			{"?", "this help"},
		},
	},
	{
		title: "About sort",
		items: []helpEntry{
			{"", "selected fields sort first, in the order shown"},
			{"", "▴ ascending · ▾ descending · time fields default to newest first"},
			{"", "name always breaks remaining ties"},
			{"", "changes apply immediately and are saved with the session"},
		},
	},
}

var helpSectionsQuickFilter = []helpSection{
	{
		title: "Quick filter",
		items: []helpEntry{
			{"type", "filter project names and metadata"},
			{"enter / tab", "apply filter"},
			{"esc", "cancel filter"},
			{"?", "this help"},
		},
	},
}

var helpSectionsDetails = []helpSection{
	{
		title: "Navigation",
		items: []helpEntry{
			{"j / k / n / p", "next / previous project"},
			{"enter / o", "select path and exit"},
			{"esc / q / h", "back to list"},
			{"right click", "back to list"},
			{"?", "this help"},
		},
	},
	{
		title: "Projects",
		items: []helpEntry{
			{"P", "copy absolute path to clipboard"},
			{"y", "yank project details as markdown"},
			{"F5 / ctrl+r", "refresh helpers"},
		},
	},
	{
		title: "Tags & annotations",
		items: []helpEntry{
			{"t", "add tag"},
			{"a", "edit annotations"},
		},
	},
	{
		title: "Mouse",
		items: []helpEntry{
			{"left click shortcuts", "run shortcut"},
		},
	},
}

var helpSectionsAdvFilterFields = []helpSection{
	{
		title: "Advanced filter",
		items: []helpEntry{
			{"j / k", "move"},
			{"enter", "choose field or clear all"},
			{"/", "filter field list"},
			{"esc", "close"},
			{"?", "this help"},
		},
	},
}

var helpSectionsAdvFilterValues = []helpSection{
	{
		title: "Filter values",
		items: []helpEntry{
			{"j / k", "move"},
			{"enter", "pick value"},
			{"(unset)", "match projects with no value"},
			{"(clear this field)", "remove filter on this field"},
			{"/", "filter value list"},
			{"esc", "back to fields"},
			{"?", "this help"},
		},
	},
}

var helpSectionsAdvFilterMulti = []helpSection{
	{
		title: "Filter values (multi)",
		items: []helpEntry{
			{"j / k", "move"},
			{"space / enter", "cycle [ ] ignore · [+] match · [-] exclude"},
			{"(apply filter)", "apply match and exclude rules"},
			{"(unset)", "match projects with no value"},
			{"(clear this field)", "remove filter on this field"},
			{"/", "filter value list"},
			{"esc", "back to fields"},
			{"?", "this help"},
		},
	},
}

var helpSectionsLayout = []helpSection{
	{
		title: "Layout menu",
		items: []helpEntry{
			{"j / k", "move"},
			{"← / → / tab", "choose an option"},
			{"space / enter", "next option"},
			{"esc / q / l", "close"},
			{"?", "this help"},
		},
	},
	{
		title: "About table mode",
		items: []helpEntry{
			{"", "one row per project with values aligned under column headers"},
			{"", "T toggles it from the list without opening this menu"},
			{"", "click a column header to make it the primary sort; click again to flip direction"},
		},
	},
}

var helpSectionsView = []helpSection{
	{
		title: "Views",
		items: []helpEntry{
			{"j / k", "move"},
			{"enter", "load view or start save"},
			{"esc / q / v", "close"},
			{"?", "this help"},
		},
	},
	{
		title: "About views",
		items: []helpEntry{
			{"", "predefined views cannot be overwritten"},
			{"", "load a preset, customize, then save as a new name"},
		},
	},
}

var helpSectionsWrite = []helpSection{
	{
		title: "Write",
		items: []helpEntry{
			{"j / k", "move"},
			{"enter", "choose export format"},
			{"esc / q / w", "close"},
			{"?", "this help"},
		},
	},
	{
		title: "Formats",
		items: []helpEntry{
			{"", "markdown · one section per project"},
			{"", "csv · all fields as columns"},
			{"", "json · structured project records"},
		},
	},
}

var helpSectionsWritePath = []helpSection{
	{
		title: "Write path",
		items: []helpEntry{
			{"type", "destination file path"},
			{"enter", "continue to clipboard option"},
			{"esc", "back to format menu"},
			{"w", "close write menu"},
			{"?", "this help"},
		},
	},
	{
		title: "Default path",
		items: []helpEntry{
			{"", "defaults to a temporary file removed on quit"},
		},
	},
}

var helpSectionsWriteClipboard = []helpSection{
	{
		title: "Copy path",
		items: []helpEntry{
			{"j / k", "move"},
			{"enter", "write file and finish"},
			{"esc", "back to path"},
			{"w", "close write menu"},
			{"?", "this help"},
		},
	},
}

var helpSectionsTagOrder = []helpSection{
	{
		title: "Tag importance",
		items: []helpEntry{
			{"j / k", "move cursor"},
			{"J / shift+↑", "move tag up"},
			{"K / shift+↓", "move tag down"},
			{"g / G", "first / last tag"},
			{"enter", "save and close"},
			{"esc / q", "cancel"},
			{"?", "this help"},
		},
	},
}

var helpSectionsAnnotateKey = []helpSection{
	{
		title: "Annotate · pick key",
		items: []helpEntry{
			{"j / k", "move"},
			{"enter", "pick key or create new key"},
			{"/", "search keys"},
			{"esc / q", "close"},
			{"?", "this help"},
		},
	},
}

var helpSectionsAnnotateNewKey = []helpSection{
	{
		title: "Annotate · new key",
		items: []helpEntry{
			{"type", "annotation key name"},
			{"enter", "continue to value"},
			{"esc", "back to key list"},
			{"?", "this help"},
		},
	},
}

var helpSectionsAnnotateValue = []helpSection{
	{
		title: "Annotate · value",
		items: []helpEntry{
			{"type", "annotation value (empty clears)"},
			{"↑ / ↓ / tab", "pick existing value"},
			{"enter", "save and close"},
			{"esc", "back or close"},
			{"?", "this help"},
		},
	},
}

var helpSectionsBulkSelect = []helpSection{
	{
		title: "Bulk select",
		items: []helpEntry{
			{"space", "toggle project in selection"},
			{"t", "tag selected projects"},
			{"a", "annotate selected projects"},
			{"F5 / ctrl+r", "refresh selected projects"},
			{"b", "exit bulk select"},
			{"?", "this help"},
		},
	},
}

var helpSectionsTagging = []helpSection{
	{
		title: "Add tag",
		items: []helpEntry{
			{"type", "tag name"},
			{"↑ / ↓ / tab", "pick existing tag"},
			{"enter", "add tag to project"},
			{"esc", "cancel"},
			{"?", "this help"},
		},
	},
}

func flattenHelpSections(sections []helpSection) []string {
	var lines []string
	for _, sec := range sections {
		lines = append(lines, "§"+sec.title)
		for _, item := range sec.items {
			lines = append(lines, fmt.Sprintf("\t%s\t%s", item.keys, item.desc))
		}
	}
	return lines
}

func (u *helpUI) rebuildLines(sections []helpSection) {
	u.lines = flattenHelpSections(sections)
	if u.cursor >= len(u.lines) {
		u.cursor = len(u.lines) - 1
	}
	if u.cursor < 0 {
		u.cursor = 0
	}
}

func (m *model) openHelp() {
	ctx := m.currentHelpContext()
	m.help.open = true
	m.help.cursor = 0
	m.help.title = helpTitleForContext(ctx)
	m.help.rebuildLines(helpSectionsForContext(ctx))
}

func (m *model) toggleHelp() {
	if m.help.open {
		m.help.open = false
		return
	}
	m.openHelp()
}

func (m model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-2, msg.Height-4)
		m.help.setSize(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		u := &m.help
		n := len(u.lines)
		switch {
		case key.Matches(msg, helpClose):
			u.open = false
			return m, nil
		case n == 0:
			return m, nil
		case key.Matches(msg, helpUp):
			if u.cursor > 0 {
				u.cursor--
			}
		case key.Matches(msg, helpDown):
			if u.cursor < n-1 {
				u.cursor++
			}
		case key.Matches(msg, helpTop):
			u.cursor = 0
		case key.Matches(msg, helpEnd):
			u.cursor = n - 1
		}
	}
	return m, nil
}

func (m model) helpView() string {
	u := m.help
	if !u.open {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	title := u.title
	if title == "" {
		title = "Help"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	avail := u.height - 5
	if avail < 3 {
		avail = 3
	}
	start := 0
	if u.cursor >= avail {
		start = u.cursor - avail + 1
	}
	end := start + avail
	if end > len(u.lines) {
		end = len(u.lines)
	}

	for i := start; i < end; i++ {
		line := u.lines[i]
		marker := "  "
		if i == u.cursor {
			marker = cur.Render("> ")
		}
		if strings.HasPrefix(line, "§") {
			secTitle := strings.TrimPrefix(line, "§")
			if i == u.cursor {
				b.WriteString(marker + cur.Render(secTitle) + "\n")
			} else {
				b.WriteString("  " + dim.Render(secTitle) + "\n")
			}
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, "\t"), "\t", 2)
		if len(parts) != 2 {
			b.WriteString(marker + line + "\n")
			continue
		}
		row := keyStyle.Render(parts[0]) + dim.Render("  "+parts[1])
		if i == u.cursor {
			b.WriteString(marker + row + "\n")
		} else {
			b.WriteString("  " + row + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dim.Render("j/k scroll · g/G jump · ? esc close"))
	return b.String()
}
