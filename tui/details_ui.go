package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jamesread/RepoManager/core/helpers"
)

// detailsUI is the full-screen view for a single project. It renders live
// from m.all, so helper results arriving in the background appear as they
// land. From here `enter`/`o` selects the project (prints its path and exits),
// which keeps the `cd "$(plist)"` workflow one extra keypress away.
type detailsUI struct {
	open   bool
	path   string
	status string
}

var (
	detailsSelect  = key.NewBinding(key.WithKeys("enter", "o"), key.WithHelp("enter", "select"))
	detailsBack    = key.NewBinding(key.WithKeys("esc", "q", "backspace", "h"), key.WithHelp("esc", "back"))
	detailsTag     = key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tag"))
	detailsAnnot   = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "annotate"))
	detailsRefresh = key.NewBinding(key.WithKeys("f5", "ctrl+r"), key.WithHelp("f5", "refresh"))
	detailsCopy    = key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "copy path"))
	detailsYank    = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank md"))
	detailsNext    = key.NewBinding(key.WithKeys("j", "down", "n"))
	detailsPrev    = key.NewBinding(key.WithKeys("k", "up", "p"))
)

type detailsHelpMap struct{}

func (detailsHelpMap) ShortHelp() []key.Binding {
	return []key.Binding{
		detailsSelect,
		key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "nav")),
		detailsCopy,
		detailsYank,
		detailsTag,
		detailsAnnot,
		detailsRefresh,
		detailsBack,
	}
}

func (detailsHelpMap) FullHelp() [][]key.Binding {
	return nil
}

func (m model) detailsHelpBar() string {
	h := help.New()
	h.Width = m.width - 2
	if h.Width < 0 {
		h.Width = 0
	}
	return m.list.Styles.HelpStyle.Render(styledShortHelpView(h, detailsHelpMap{}.ShortHelp()))
}

func (m *model) visibleDetailsHelpBindingAtX(x int) (key.Binding, bool) {
	bindings := detailsHelpMap{}.ShortHelp()
	helpModel := help.New()
	separator := helpModel.Styles.ShortSeparator.Inline(true).Render(helpModel.ShortSeparator)

	totalWidth := 0
	width := m.width - 2
	if width < 0 {
		width = 0
	}

	for _, kb := range bindings {
		if !kb.Enabled() {
			continue
		}

		sep := ""
		if totalWidth > 0 {
			sep = separator
		}
		item := sep + styledShortHelpItem(kb, helpModel.Styles)
		w := lipgloss.Width(item)

		if width > 0 && totalWidth+w > width {
			tail := " " + helpModel.Styles.Ellipsis.Inline(true).Render(helpModel.Ellipsis)
			if totalWidth+lipgloss.Width(tail) < width {
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

func (m *model) invokeDetailsHelpBinding(b key.Binding) tea.Cmd {
	for _, keyStr := range b.Keys() {
		msg, ok := keyMsgFromString(keyStr)
		if !ok {
			continue
		}
		cmd, handled := m.detailsHandleKey(msg)
		if handled {
			return cmd
		}
	}
	return nil
}

func (m *model) detailsHandleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	p, ok := m.detailsProject()
	switch {
	case key.Matches(msg, detailsBack):
		m.details.open = false
		m.details.status = ""
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
		m.writeMenu.cleanup()
		m.persistSessionForce()
		m.quitting = true
		if m.eng != nil {
			m.eng.Close()
		}
		return tea.Quit, true
	case key.Matches(msg, detailsSelect):
		if !ok {
			return nil, true
		}
		m.writeMenu.cleanup()
		m.persistSessionForce()
		m.selected = p.path
		m.quitting = true
		if m.eng != nil {
			m.eng.Close()
		}
		return tea.Quit, true
	case key.Matches(msg, detailsNext):
		m.details.status = ""
		m.detailsStep(+1)
		return nil, true
	case key.Matches(msg, detailsPrev):
		m.details.status = ""
		m.detailsStep(-1)
		return nil, true
	case key.Matches(msg, detailsCopy):
		if ok {
			return m.copyProjectPath(p.path), true
		}
	case key.Matches(msg, detailsYank):
		if ok {
			return m.yankProjectDetails(p), true
		}
	case key.Matches(msg, detailsRefresh):
		if ok {
			return m.startRefresh(p), true
		}
	case key.Matches(msg, detailsTag):
		if ok {
			return m.openTagging([]projectItem{p}), true
		}
	case key.Matches(msg, detailsAnnot):
		if ok {
			return m.openAnnotate([]projectItem{p}), true
		}
	}
	return nil, false
}

func (m *model) handleDetailsMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	if msg.Action != tea.MouseActionPress && msg.Action != tea.MouseActionRelease {
		return nil, false
	}

	if msg.Button == tea.MouseButtonRight {
		return m.invokeDetailsHelpBinding(detailsBack), true
	}

	if msg.Button != tea.MouseButtonLeft {
		return nil, false
	}

	helpH := lipgloss.Height(m.detailsHelpBar())
	totalH := lipgloss.Height(m.detailsView())
	helpStart := totalH - helpH
	if msg.Y < helpStart || msg.Y >= totalH {
		return nil, false
	}

	if kb, ok := m.visibleDetailsHelpBindingAtX(msg.X); ok {
		return m.invokeDetailsHelpBinding(kb), true
	}
	return nil, true
}

func (m *model) openDetails(path string) {
	m.details.open = true
	m.details.path = path
}

// detailsProject returns the live project record for the details view.
func (m *model) detailsProject() (projectItem, bool) {
	for _, p := range m.all {
		if p.path == m.details.path {
			return p, true
		}
	}
	return projectItem{}, false
}

// detailsStep moves the details view to the neighbouring project in the
// currently visible (filtered, sorted) list and keeps the list cursor in sync.
func (m *model) detailsStep(delta int) {
	items := m.list.Items()
	if len(items) == 0 {
		return
	}
	idx := -1
	for i, it := range items {
		if p, ok := it.(projectItem); ok && p.path == m.details.path {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = m.list.Index()
	}
	next := idx + delta
	if next < 0 || next >= len(items) {
		return
	}
	if p, ok := items[next].(projectItem); ok {
		m.details.path = p.path
		m.list.Select(next)
	}
}

func (m model) updateDetails(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tagAddedMsg:
		if msg.err == nil {
			for _, path := range msg.paths {
				m.refreshProjectTags(path)
			}
		}
		return m, nil
	case annotationSetMsg:
		return m, m.handleAnnotationSet(msg)
	case clipboardMsg:
		if msg.err != nil {
			m.details.status = fmt.Sprintf("clipboard: %v", msg.err)
		} else {
			m.details.status = msg.status
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-2, msg.Height-4)
		m.annotate.setSize(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.MouseMsg:
		if cmd, handled := m.handleMouseWheel(msg); handled {
			return m, cmd
		}
		if cmd, handled := m.handleDetailsMouse(msg); handled {
			return m, cmd
		}
	case tea.KeyMsg:
		if cmd, handled := m.detailsHandleKey(msg); handled {
			return m, cmd
		}
	}
	return m, nil
}

func (m model) detailsView() string {
	p, ok := m.detailsProject()
	if !ok {
		return "\n" + titleStyle.Render("Project") + "\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("project no longer present · esc back")
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(18)
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).MarginTop(1)

	width := m.width - 2
	if width < 40 {
		width = 40
	}
	row := func(k, v string) string {
		if v == "" {
			v = dim.Render("—")
		}
		return label.Render(k) + ansi.Truncate(v, width-18, "…")
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render(p.rel))
	b.WriteString("\n")
	b.WriteString(dim.Render(filepath.Clean(p.path)))
	b.WriteString("\n")
	if ghURL := helpers.GitHubPageURLFromMeta(p.meta); ghURL != "" {
		b.WriteString(dim.Render(ghURL))
		b.WriteString("\n")
	}

	// Tags — all of them, in importance order, no cap on this screen.
	b.WriteString(section.Render("Tags"))
	b.WriteString("\n")
	if len(p.tags) == 0 {
		b.WriteString(dim.Render("none"))
	} else {
		chips := make([]string, 0, len(p.tags))
		for _, tag := range m.store.SortTagsByImportance(p.tags) {
			bg := m.store.ColorForTag(tag)
			chips = append(chips, tagChipStyle.
				Foreground(lipgloss.Color(contrastingForeground(bg))).
				Background(lipgloss.Color(bg)).
				Render(tag))
		}
		b.WriteString(strings.Join(chips, " "))
	}
	b.WriteString("\n")

	// Annotations — every global key, so unset ones are visible and actionable.
	keys := m.store.AnnotationKeys()
	if len(keys) > 0 || len(p.annotations) > 0 {
		b.WriteString(section.Render("Annotations"))
		b.WriteString("\n")
		seen := map[string]bool{}
		for _, k := range keys {
			seen[k] = true
			v := p.annotations[k]
			if v == "" {
				b.WriteString(row("@"+k, dim.Render("unset")))
			} else {
				b.WriteString(row("@"+k, annotationStyle.Render(v)))
			}
			b.WriteString("\n")
		}
		var extra []string
		for k := range p.annotations {
			if !seen[k] && p.annotations[k] != "" {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		for _, k := range extra {
			b.WriteString(row("@"+k, annotationStyle.Render(p.annotations[k])))
			b.WriteString("\n")
		}
	}

	// Helper metadata, grouped in registration order; times show relative +
	// absolute so the page is useful without hovering.
	b.WriteString(section.Render("Metadata"))
	b.WriteString("\n")
	shown := 0
	for _, f := range m.helperFields() {
		if f.ID == "name" || f.ID == "tags" {
			continue
		}
		v, present := p.meta[f.ID]
		if !present {
			continue
		}
		b.WriteString(row(f.Label, formatDetailValue(v)))
		b.WriteString("\n")
		shown++
	}
	if shown == 0 {
		if m.scanning {
			b.WriteString(dim.Render("collecting…"))
		} else {
			b.WriteString(dim.Render("none"))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.details.status != "" {
		b.WriteString(dim.Render(m.details.status))
		b.WriteString("\n")
	}
	if m.tagging {
		b.WriteString(m.renderTagPrompt())
	}
	b.WriteString(m.detailsHelpBar())
	return b.String()
}

// formatDetailValue is helpers.FormatMetaValue with fuller output for the details page.
func formatDetailValue(v any) string {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return fmt.Sprintf("%s  (%s)", helpers.RelativeTime(t), t.Local().Format("2006-01-02 15:04"))
	case []string:
		if len(t) == 0 {
			return ""
		}
		return strings.Join(t, ", ")
	default:
		return helpers.FormatMetaValue(v)
	}
}
