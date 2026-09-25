package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/project"
	"github.com/jamesread/RepoManager/core/store"
)

// Table mode draws each project on one line with values aligned in columns
// and a header row naming each column. Column widths are recomputed whenever
// the list is republished or resized. The state is shared by pointer between
// the model and the list delegate, like refreshState.

const (
	tableColGap      = 2
	tableRowPrefix   = 2 // border/padding cells before the first column
	tableMaxColWidth = 40
	tableMinColWidth = 6
	tableColPath     = "path"
)

// tableSortMarkerWidth reserves room in every header for a direction glyph.
const tableSortMarkerWidth = 2

type tableColumn struct {
	id    string
	label string
	width int
}

type tableState struct {
	enabled bool
	columns []tableColumn
}

func (m *model) tableEnabled() bool {
	return m.table != nil && m.table.enabled
}

func (m *model) tableHeaderLines() int {
	if m.tableEnabled() {
		return 1
	}
	return 0
}

// resizeList sizes the bubbles list, reserving a line for the table header
// when table mode is on.
func (m *model) resizeList(w, h int) {
	h -= m.tableHeaderLines()
	if h < 1 {
		h = 1
	}
	m.list.SetSize(w, h)
	m.recomputeTableColumns()
}

// tableCellText renders the plain (or, for tags, chip-styled) value of one
// column for a project.
func tableCellText(st *store.Store, root string, p projectItem, id string) string {
	switch id {
	case "name":
		return p.rel
	case tableColPath:
		if st == nil {
			return p.path
		}
		return project.FormatPath(p.path, root, st.PathRelative(), st.PathTilde())
	case "tags":
		return renderProjectTags(st, p.tags)
	}
	return helpers.FormatMetaValue(p.fieldValue(id))
}

func (m *model) visibleProjects() []projectItem {
	items := m.list.VisibleItems()
	out := make([]projectItem, 0, len(items))
	for _, it := range items {
		if p, ok := it.(projectItem); ok {
			out = append(out, p)
		}
	}
	return out
}

// recomputeTableColumns rebuilds the column set and widths from the visible
// projects. Data columns only appear when at least one project has a value.
func (m *model) recomputeTableColumns() {
	if !m.tableEnabled() {
		return
	}
	projects := m.visibleProjects()

	cols := []tableColumn{
		{id: "name", label: "Name"},
		{id: tableColPath, label: "Path"},
	}
	if m.eng != nil {
		for _, f := range m.allFields() {
			if f.ID == "name" || f.ID == "tags" {
				continue
			}
			for _, p := range projects {
				if p.fieldValue(f.ID) != nil {
					cols = append(cols, tableColumn{id: f.ID, label: f.Label})
					break
				}
			}
		}
	}
	for _, p := range projects {
		if len(p.tags) > 0 {
			cols = append(cols, tableColumn{id: "tags", label: "Tags"})
			break
		}
	}

	for i := range cols {
		w := lipgloss.Width(cols[i].label) + tableSortMarkerWidth
		for _, p := range projects {
			if cw := lipgloss.Width(tableCellText(m.store, m.root, p, cols[i].id)); cw > w {
				w = cw
			}
		}
		if cols[i].id == "name" {
			if m.bulk.isActive() {
				w += lipgloss.Width(tickMark(false)) + 1
			}
			w += lipgloss.Width(" " + refreshGlyph)
		}
		if w > tableMaxColWidth {
			w = tableMaxColWidth
		}
		cols[i].width = w
	}

	m.table.columns = fitTableColumns(cols, m.list.Width()-tableRowPrefix)
}

func tableColumnsWidth(cols []tableColumn) int {
	total := 0
	for i, c := range cols {
		if i > 0 {
			total += tableColGap
		}
		total += c.width
	}
	return total
}

// fitTableColumns shrinks the widest columns first until the row fits, then
// drops trailing columns that still cannot fit. Name is always kept.
func fitTableColumns(cols []tableColumn, avail int) []tableColumn {
	if avail <= 0 || len(cols) == 0 {
		return cols
	}
	for tableColumnsWidth(cols) > avail {
		widest := -1
		for i, c := range cols {
			if c.width <= tableMinColWidth {
				continue
			}
			if widest < 0 || c.width > cols[widest].width {
				widest = i
			}
		}
		if widest < 0 {
			break
		}
		cols[widest].width--
	}
	for len(cols) > 1 && tableColumnsWidth(cols) > avail {
		cols = cols[:len(cols)-1]
	}
	return cols
}

// padCell truncates or right-pads text to width, preserving ANSI styling.
func padCell(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) > width {
		text = ansi.Truncate(text, width, "…")
	}
	if pad := width - lipgloss.Width(text); pad > 0 {
		text += strings.Repeat(" ", pad)
	}
	return text
}

// tableHeaderLine renders the column header row. Every column taking part in
// the sort shows its direction glyph; the primary key is highlighted.
func (m *model) tableHeaderLine() string {
	if !m.tableEnabled() {
		return ""
	}
	keys := m.sortKeyList()
	byID := make(map[string]sortKey, len(keys))
	for _, k := range keys {
		byID[k.Field.ID] = k
	}
	primary := keys[0].Field.ID
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	cells := make([]string, 0, len(m.table.columns))
	for _, col := range m.table.columns {
		label := col.label
		style := metaStyle
		if k, ok := byID[col.id]; ok {
			label += " " + k.glyph()
			if col.id == primary {
				style = active
			}
		}
		cells = append(cells, style.Render(padCell(label, col.width)))
	}
	return strings.Repeat(" ", tableRowPrefix) + strings.Join(cells, strings.Repeat(" ", tableColGap))
}

// tableColumnAtX maps a screen column to a table column.
func (m *model) tableColumnAtX(x int) (tableColumn, bool) {
	if !m.tableEnabled() {
		return tableColumn{}, false
	}
	pos := tableRowPrefix
	for _, col := range m.table.columns {
		if x >= pos && x < pos+col.width {
			return col, true
		}
		pos += col.width + tableColGap
	}
	return tableColumn{}, false
}

// setTableMode switches table rendering on or off and re-lays out the list.
func (m *model) setTableMode(on bool) tea.Cmd {
	if m.table == nil {
		m.table = &tableState{}
		m.applyLayout(m.layout)
	}
	if m.table.enabled == on {
		return nil
	}
	m.table.enabled = on
	if err := m.engSend(engine.SetTableModeRequest{Enabled: on}); err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("table error: %v", err))
	}
	if m.width > 0 && m.height > 0 {
		m.resizeList(m.width-2, m.height-4)
	} else {
		m.recomputeTableColumns()
	}
	m.publishList()
	m.refreshTitle()
	if on {
		return m.list.NewStatusMessage("table mode · on")
	}
	return m.list.NewStatusMessage("table mode · off")
}

// toggleTableMode is the `T` quick shortcut.
func (m *model) toggleTableMode() tea.Cmd {
	return m.setTableMode(!m.tableEnabled())
}

// handleTableHeaderClick makes the clicked column the primary sort field, or
// flips its direction when it is already primary.
func (m *model) handleTableHeaderClick(x int) tea.Cmd {
	col, ok := m.tableColumnAtX(x)
	if !ok {
		return nil
	}
	field, ok := m.fieldByID(col.id)
	if !ok || !field.Sortable {
		return m.list.NewStatusMessage(fmt.Sprintf("%s is not sortable", col.label))
	}
	if m.currentSort().ID == field.ID && len(m.resolvedSortKeys()) > 0 {
		m.toggleSortDirection(field.ID)
	} else {
		m.setPrimarySort(field.ID)
	}
	return m.applySortChange()
}

// renderTableRow draws one project as aligned cells.
func (d itemDelegate) renderTableRow(w io.Writer, m list.Model, index int, project projectItem) {
	s := &d.Styles
	isCursor := index == m.Index()
	bulkChecked := d.bulk.isActive() && d.bulk.isSelected(project.path)

	cellStyle := s.NormalTitle.Inline(true)
	if isCursor && m.FilterState() != list.Filtering {
		cellStyle = s.SelectedTitle.Inline(true)
	}
	if bulkChecked {
		cellStyle = cellStyle.Bold(true)
	}
	if m.FilterState() == list.Filtering && m.FilterValue() == "" {
		cellStyle = s.DimmedTitle.Inline(true)
	}

	prefix := strings.Repeat(" ", tableRowPrefix)
	if isCursor {
		border := lipgloss.NewStyle().Foreground(s.SelectedTitle.GetBorderLeftForeground())
		prefix = border.Render("│") + " "
	}

	cells := make([]string, 0, len(d.table.columns))
	for _, col := range d.table.columns {
		text := tableCellText(d.store, d.root, project, col.id)
		switch col.id {
		case "tags":
			cells = append(cells, padCell(text, col.width))
			continue
		case "name":
			if d.bulk.isActive() {
				text = tickMark(bulkChecked) + " " + text
			}
			if d.refresh.active(project.path) {
				text += " " + refreshGlyph
			}
		}
		cells = append(cells, cellStyle.Render(padCell(text, col.width)))
	}
	fmt.Fprint(w, prefix+strings.Join(cells, strings.Repeat(" ", tableColGap)))
}
