package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/project"
)

const titleAppName = "RepoManager"
const listEmptyMessage = "no projects in the list"

// filterFieldShort returns a compact label for filter status in the title bar.
func filterFieldShort(fieldID string) string {
	if key, ok := annotationKeyFromField(fieldID); ok {
		return "@" + key
	}
	switch fieldID {
	case "tags":
		return "#"
	case "name":
		return "name"
	case helpers.FieldDirMtime:
		return "mtime"
	case helpers.FieldGitLastCommit:
		return "git"
	case helpers.FieldGitHubOwner:
		return "gh.o"
	case helpers.FieldGitHubRepo:
		return "gh.r"
	case helpers.FieldGitHubFullName:
		return "gh"
	case helpers.FieldGitHubTopics:
		return "gh.top"
	case helpers.FieldGitHubOpenIssues:
		return "gh.iss"
	case helpers.FieldGitHubOpenPRs:
		return "gh.pr"
	case helpers.FieldGitHubIsFork:
		return "gh.fork"
	case helpers.FieldModules:
		return "mods"
	default:
		if strings.HasPrefix(fieldID, "github.") {
			return "gh." + strings.TrimPrefix(fieldID, "github.")
		}
		return fieldID
	}
}

func (r filterRule) shortSummary() string {
	label := filterFieldShort(r.FieldID)
	if r.Unset {
		return label + "∅"
	}
	pos := r.activeValues()
	neg := r.activeNegated()
	if len(pos) == 0 && len(neg) == 0 {
		return label + "∅"
	}
	var parts []string
	if len(pos) > 1 {
		parts = append(parts, label+"="+strings.Join(pos, "+"))
	} else if len(pos) == 1 {
		parts = append(parts, label+"="+pos[0])
	}
	if len(neg) > 1 {
		parts = append(parts, label+"!"+strings.Join(neg, "+"))
	} else if len(neg) == 1 {
		parts = append(parts, label+"!"+neg[0])
	}
	return strings.Join(parts, "·")
}

func (m *model) filterStatusRaw() string {
	var parts []string
	if m.list.FilterState() == list.FilterApplied {
		if q := strings.TrimSpace(m.list.FilterValue()); q != "" {
			parts = append(parts, "/"+q)
		}
	}
	for _, r := range m.advFilter.rules {
		parts = append(parts, r.shortSummary())
	}
	return strings.Join(parts, " · ")
}

func truncateTitleSection(text string, maxWidth int) string {
	if text == "" || maxWidth <= 0 {
		return text
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	return ansi.Truncate(text, maxWidth, "…")
}

func (m *model) refreshTitle() {
	cwd := project.FormatScanRoot(m.root, m.store.PathTilde())
	sortLabel := m.sortTitleLabel()

	width := m.width
	if width <= 0 {
		width = 80
	}

	prefix := titleAppName + " · " + cwd
	scanPart := ""
	if m.scanning {
		scanPart = " · scanning…"
	}
	if m.eng != nil {
		if n := m.eng.GitHubPendingCount(); n > 0 {
			scanPart += fmt.Sprintf(" · GH %d", n)
		}
	}
	sortPart := " · sort " + sortLabel
	rightFixed := scanPart + sortPart
	if m.bulk.isActive() {
		rightFixed += fmt.Sprintf(" · bulk %d", m.bulk.count())
	}

	m.titleLeft = prefix
	if raw := m.filterStatusRaw(); raw != "" {
		maxFilter := width / 2
		fixed := lipgloss.Width(prefix + rightFixed)
		budget := width - fixed - lipgloss.Width(" · ")
		if budget > maxFilter {
			budget = maxFilter
		}
		if budget < 6 {
			budget = 6
		}
		m.titleLeft += " · " + truncateTitleSection(raw, budget)
	}
	if lipgloss.Width(m.titleLeft) > width-lipgloss.Width(rightFixed) {
		m.titleLeft = truncateTitleSection(m.titleLeft, width-lipgloss.Width(rightFixed))
	}
	m.titleLeft += rightFixed
	m.list.Title = m.titleLeft
}

func extractTitleSuffix(originalLine, titleLeft string) string {
	if titleLeft == "" {
		return ""
	}
	plain := ansi.Strip(originalLine)
	idx := strings.Index(plain, titleLeft)
	if idx < 0 {
		return ""
	}
	return plain[idx+len(titleLeft):]
}

func (m *model) composeTitleInner() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	leftPlain := m.titleLeft
	leftStyled := titleStyle.Render(leftPlain)
	viewName := m.activeViewTitle()
	if viewName == "" {
		return leftStyled
	}

	viewLabel := viewTitleStyle.Render("view: " + viewName)
	leftW := lipgloss.Width(leftStyled)
	viewW := lipgloss.Width(viewLabel)
	gap := width - leftW - viewW
	if gap < 2 {
		maxPlain := width - viewW - 2
		if maxPlain < 10 {
			maxPlain = 10
		}
		leftPlain = truncateTitleSection(leftPlain, maxPlain)
		leftStyled = titleStyle.Render(leftPlain)
		leftW = lipgloss.Width(leftStyled)
		gap = width - leftW - viewW
		if gap < 1 {
			gap = 1
		}
	}
	return leftStyled + strings.Repeat(" ", gap) + viewLabel
}

func (m model) listPaginationSummary() string {
	total := len(m.list.VisibleItems())
	page := m.list.Paginator.Page + 1
	pages := m.list.Paginator.TotalPages
	noun := "projects"
	if total == 1 {
		noun = "project"
	}
	return fmt.Sprintf("page %d of %d · %d %s", page, pages, total, noun)
}

func (m model) listPaginationBlock() string {
	l := m.list
	if !l.ShowPagination() || l.Paginator.TotalPages < 2 {
		return ""
	}
	pager := l.Paginator.View()
	if ansi.StringWidth(pager) > l.Width() {
		pager = l.Styles.ArabicPagination.Render(l.Paginator.View())
	}
	summary := metaStyle.Render(m.listPaginationSummary())
	pagerWidth := lipgloss.Width(pager)
	line := pager + lipgloss.NewStyle().
		Width(l.Width()-pagerWidth).
		Align(lipgloss.Right).
		Render(summary)

	style := l.Styles.PaginationStyle
	_, spacing := m.itemDelegateSize()
	if spacing == 0 && style.GetMarginTop() == 0 {
		style = style.MarginTop(1)
	}
	return style.Render(line)
}

func (m model) listEmptyBlock(width, height int) string {
	if len(m.list.VisibleItems()) > 0 || m.list.FilterState() == list.Filtering {
		return ""
	}
	if width <= 0 {
		width = m.width
	}
	if height < 1 {
		height = 1
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, metaStyle.Render(listEmptyMessage))
}

func (m *model) applyListEmptyBlock(lines []string, regions listViewRegions) {
	itemHeight := regions.itemsEnd - regions.itemsStart
	if itemHeight < 1 {
		return
	}
	empty := m.listEmptyBlock(m.list.Width(), itemHeight)
	if empty == "" {
		return
	}
	emptyLines := strings.Split(empty, "\n")
	for i := 0; i < itemHeight && regions.itemsStart+i < len(lines); i++ {
		line := ""
		if i < len(emptyLines) {
			line = emptyLines[i]
		}
		lines[regions.itemsStart+i] = line
	}
}

// composeTitleBar re-renders only the first line of the list's title bar; the
// bar's bottom padding line is left in place by the caller.
func (m *model) composeTitleBar(originalLine string) string {
	suffix := extractTitleSuffix(originalLine, m.titleLeft)
	return m.list.Styles.TitleBar.PaddingBottom(0).Render(m.composeTitleInner() + suffix)
}

func (m model) mainListView() string {
	raw := m.list.View()
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return raw
	}

	regions := m.listViewRegions()
	if regions.headerLines > 0 {
		at := regions.titleLines
		if at > len(lines) {
			at = len(lines)
		}
		lines = append(lines[:at], append([]string{m.tableHeaderLine()}, lines[at:]...)...)
	}
	m.applyListEmptyBlock(lines, regions)
	if regions.titleLines > 0 && m.list.ShowTitle() && m.list.FilterState() != list.Filtering {
		lines[0] = m.composeTitleBar(lines[0])
	}

	if regions.helpLines > 0 && regions.helpStart < len(lines) {
		helpText := m.list.Styles.HelpStyle.Render(
			styledShortHelpView(m.list.Help, m.list.ShortHelp()),
		)
		end := regions.helpStart + regions.helpLines
		if end > len(lines) {
			end = len(lines)
		}
		helpLines := strings.Split(helpText, "\n")
		for i := 0; i < len(helpLines) && regions.helpStart+i < end; i++ {
			lines[regions.helpStart+i] = helpLines[i]
		}
	}

	if pag := m.listPaginationBlock(); pag != "" {
		pagLines := lipgloss.Height(pag)
		insertAt := regions.helpStart - pagLines
		if insertAt < regions.itemsStart {
			insertAt = regions.itemsStart
		}
		if insertAt < 0 {
			insertAt = 0
		}
		if insertAt+pagLines > len(lines) {
			lines = append(lines, make([]string, insertAt+pagLines-len(lines))...)
		}
		for i, pl := range strings.Split(pag, "\n") {
			if insertAt+i < len(lines) {
				lines[insertAt+i] = pl
			}
		}
	}

	return strings.Join(lines, "\n")
}
