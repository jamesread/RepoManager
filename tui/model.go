// Package tui implements the interactive project list: the Bubble Tea model,
// its screens (details, filters, sort, layout, views, tags, annotations,
// bulk select, write/export, help) and the list rendering delegate.
package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/project"
	"github.com/jamesread/RepoManager/core/store"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))
	tagChipStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)
	tagOverflowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
	annotationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("178"))
	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	viewTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("178"))
	// bulkPickedStyle marks the tickbox of a project picked in bulk select.
	bulkPickedStyle = lipgloss.NewStyle().Bold(true)
)

type projectItem struct {
	path        string
	rel         string
	tags        []string
	meta        map[string]any
	annotations map[string]string
}

func (i projectItem) Title() string       { return i.rel }
func (i projectItem) Description() string { return i.path }
func (i projectItem) FilterValue() string {
	parts := []string{i.rel, strings.Join(i.tags, " "), i.path}
	for k, v := range i.annotations {
		parts = append(parts, k, v)
	}
	for id, v := range i.meta {
		if id == helpers.MetaKeyTags {
			continue
		}
		if s := helpers.FormatMetaValue(v); s != "" {
			parts = append(parts, id, s)
		}
	}
	return strings.Join(parts, " ")
}

func (i projectItem) metaValue(id string) any {
	if i.meta == nil {
		return nil
	}
	return i.meta[id]
}

type projectFoundMsg projectItem // kept for tests; prefer engine events

type scanFinishedMsg struct {
	err error
}

type tagAddedMsg struct {
	paths []string
	tag   string
	err   error
}

// engineEventMsg wraps an engine.Event for the Bubble Tea loop.
type engineEventMsg struct {
	Event engine.Event
}

type model struct {
	list             list.Model
	all              []projectItem
	eng              engine.Engine
	store            *store.Store
	root             string
	events           <-chan engine.Event
	selected         string
	found            int
	scanning         bool
	quitting         bool
	err              error
	tagInput         textinput.Model
	tagging          bool
	tagSuggestions   []valueCount
	tagSugIdx        int
	tagTargets       []projectItem
	sortSpecs        []store.SortSpec
	sortMenu         sortUI
	table            *tableState
	advFilter        advancedFilter
	tagOrder         tagOrderUI
	annotate         annotateUI
	details          detailsUI
	layout           layoutMode
	layoutMenu       layoutUI
	viewMenu         viewUI
	writeMenu        writeUI
	activeViewID     string
	help             helpUI
	refresh          *refreshState
	bulk             *bulkSelectState
	listDirty        bool
	lastSession      store.RootSession
	titleLeft        string
	restoringSession bool
	width            int
	height           int
}

type itemDelegate struct {
	list.DefaultDelegate
	store   *store.Store
	fields  func() []helpers.Field
	layout  layoutMode
	refresh *refreshState
	root    string
	bulk    *bulkSelectState
	table   *tableState
}

func newItemDelegate(st *store.Store, fields func() []helpers.Field, layout layoutMode, refresh *refreshState, root string, bulk *bulkSelectState) itemDelegate {
	d := itemDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
		store:           st,
		fields:          fields,
		layout:          layout,
		refresh:         refresh,
		root:            root,
		bulk:            bulk,
	}
	switch layout {
	case layoutCompact:
		d.ShowDescription = false
		d.SetHeight(1)
		d.SetSpacing(0)
	default:
		d.ShowDescription = true
		d.SetHeight(2)
		d.SetSpacing(1)
	}
	return d
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	project, ok := item.(projectItem)
	if !ok || m.Width() <= 0 {
		return
	}
	if d.table != nil && d.table.enabled {
		d.renderTableRow(w, m, index, project)
		return
	}

	s := &d.Styles
	padLeft := s.NormalTitle.GetPaddingLeft()
	padRight := s.NormalTitle.GetPaddingRight()
	textwidth := m.Width() - padLeft - padRight

	tags := d.renderTags(project.tags)
	tagsWidth := lipgloss.Width(tags)
	titleBudget := textwidth
	if tagsWidth > 0 {
		titleBudget = textwidth - tagsWidth - 1
		if titleBudget < 8 {
			titleBudget = 8
		}
	}

	// Refresh indicator sits beside the name and shares its width budget.
	indicator := ""
	if d.refresh.active(project.path) {
		indicator = " " + refreshGlyph
		titleBudget -= lipgloss.Width(indicator)
		if titleBudget < 4 {
			titleBudget = 4
		}
	}

	title := ansi.Truncate(project.rel, titleBudget, "…")

	isCursor := index == m.Index()
	bulkChecked := d.bulk.isActive() && d.bulk.isSelected(project.path)
	checkPrefix := ""
	if d.bulk.isActive() {
		checkPrefix = tickMark(bulkChecked) + " "
		titleBudget -= lipgloss.Width(checkPrefix)
		if titleBudget < 4 {
			titleBudget = 4
		}
		title = ansi.Truncate(project.rel, titleBudget, "…")
	}

	emptyFilter := m.FilterState() == list.Filtering && m.FilterValue() == ""
	isFiltered := m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied

	// Cursor keeps the list's selected colours; bulk-picked rows are bold.
	// A picked row under the cursor shows both.
	titleStyle := s.NormalTitle
	descStyle := s.NormalDesc
	if isCursor && m.FilterState() != list.Filtering {
		titleStyle = s.SelectedTitle
		descStyle = s.SelectedDesc
	}
	if bulkChecked {
		titleStyle = titleStyle.Bold(true)
		descStyle = descStyle.Bold(true)
	}
	if emptyFilter {
		titleStyle = s.DimmedTitle
		descStyle = s.DimmedDesc
	}

	if isFiltered && !emptyFilter {
		unmatched := titleStyle.Inline(true)
		matched := unmatched.Inherit(s.FilterMatch)
		title = lipgloss.StyleRunes(title, m.MatchesForItem(index), matched, unmatched)
	}
	title = titleStyle.Render(title)

	if checkPrefix != "" {
		if bulkChecked {
			checkPrefix = bulkPickedStyle.Render(checkPrefix)
		}
		title = checkPrefix + title
	}

	if indicator != "" {
		title += indicator
	}

	descText := d.formatPath(project.path)
	if meta := d.formatProjectMeta(project); meta != "" {
		descText = descText + "  " + meta
	}
	desc := descStyle.Render(ansi.Truncate(descText, textwidth, "…"))

	titleRow := title
	if tagsWidth > 0 {
		gap := m.Width() - lipgloss.Width(title) - tagsWidth
		if gap < 1 {
			gap = 1
		}
		titleRow = title + strings.Repeat(" ", gap) + tags
	}

	if d.layout == layoutCompact {
		fmt.Fprint(w, titleRow)
		return
	}
	fmt.Fprintf(w, "%s\n%s", titleRow, desc)
}

func (d itemDelegate) formatPath(absPath string) string {
	if d.store == nil {
		return absPath
	}
	return project.FormatPath(absPath, d.root, d.store.PathRelative(), d.store.PathTilde())
}

func (d itemDelegate) formatProjectMeta(p projectItem) string {
	if d.fields == nil {
		return ""
	}
	var parts []string
	if d.store != nil {
		if s := annotationSummary(d.store.AnnotationKeys(), p.annotations); s != "" {
			parts = append(parts, annotationStyle.Render(s))
		}
	}
	for _, field := range d.fields() {
		if field.ID == "name" || field.ID == "tags" {
			continue
		}
		v, ok := p.meta[field.ID]
		if !ok {
			continue
		}
		if s := helpers.FormatMetaValue(v); s != "" {
			parts = append(parts, metaStyle.Render(field.Label+": "+s))
		}
	}
	return strings.Join(parts, "  ")
}

func (d itemDelegate) renderTags(tags []string) string {
	return renderProjectTags(d.store, tags)
}

func waitEvent(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return engineEventMsg{Event: engine.ScanFinished{}}
		}
		return engineEventMsg{Event: msg}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.list.StartSpinner(), waitEvent(m.events))
}

func (m *model) handleAppKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.list.FilterState() == list.Filtering {
		return nil, false
	}
	if cmd, handled := m.handleBulkToggleKey(msg); handled {
		return cmd, true
	}
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("b"))):
		return m.toggleBulkSelect(), true
	case key.Matches(msg, key.NewBinding(key.WithKeys("F", "ctrl+f"))):
		m.openAdvancedFilter()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("l"))):
		m.openLayoutMenu()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("v"))):
		m.openViewMenu()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("w"))):
		m.openWriteMenu()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("h"))):
		m.openHelp()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("O"))):
		m.openTagOrder()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("S"))):
		m.openSortMenu()
		return nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("T"))):
		return m.toggleTableMode(), true
	case key.Matches(msg, key.NewBinding(key.WithKeys("C"))):
		return m.toggleCompactLayout(), true
	case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
		if items := m.operationTargets(); len(items) > 0 {
			return m.openAnnotate(items), true
		}
	case msg.Type == tea.KeyF5 || key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+r"))):
		if items := m.operationTargets(); len(items) > 0 {
			return m.startRefreshItems(items), true
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		m.cycleSort()
		return m.list.NewStatusMessage(fmt.Sprintf("sort · %s", m.currentSort().Label)), true
	case key.Matches(msg, key.NewBinding(key.WithKeys("t"))):
		if items := m.operationTargets(); len(items) > 0 {
			return m.openTagging(items), true
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if item, ok := m.list.SelectedItem().(projectItem); ok {
			m.openDetails(item.path)
			return nil, true
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("P"))):
		if item, ok := m.list.SelectedItem().(projectItem); ok {
			return m.copyProjectPath(item.path), true
		}
	case key.Matches(msg, detailsYank):
		if item, ok := m.list.SelectedItem().(projectItem); ok {
			return m.yankProjectDetails(item), true
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"))):
		m.writeMenu.cleanup()
		m.persistSessionForce()
		m.quitting = true
		if m.eng != nil {
			m.eng.Close()
		}
		return tea.Quit, true
	}
	return nil, false
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m, cmd, ok := m.handleSuspend(msg); ok {
		return m, cmd
	}
	if msg, ok := msg.(tea.KeyMsg); ok && key.Matches(msg, helpToggle) {
		m.toggleHelp()
		return m, nil
	}
	if msg, ok := msg.(tea.MouseMsg); ok {
		if cmd, handled := m.handleMouseWheel(msg); handled {
			return m, cmd
		}
	}
	if m.help.open {
		return m.updateHelp(msg)
	}
	if m.advFilter.open {
		return m.updateAdvancedFilter(msg)
	}
	if m.layoutMenu.open {
		return m.updateLayoutMenu(msg)
	}
	if m.viewMenu.open {
		return m.updateViewMenu(msg)
	}
	if m.writeMenu.open {
		return m.updateWriteMenu(msg)
	}
	if m.sortMenu.open {
		return m.updateSortMenu(msg)
	}
	if m.tagOrder.open {
		return m.updateTagOrder(msg)
	}
	if m.annotate.open {
		return m.updateAnnotate(msg)
	}
	if m.tagging {
		return m.updateTagging(msg)
	}
	if m.details.open {
		return m.updateDetails(msg)
	}

	switch msg := msg.(type) {
	case engineEventMsg:
		cmd := m.handleEngineEvent(msg.Event)
		if finished, ok := msg.Event.(engine.ScanFinished); ok {
			var status tea.Cmd
			if finished.Err != "" {
				status = m.list.NewStatusMessage("scan error: " + finished.Err)
			} else if m.found == 0 {
				status = m.list.NewStatusMessage("no project folders found")
			} else {
				status = m.list.NewStatusMessage(fmt.Sprintf("scan complete · %d projects", m.found))
			}
			return m, tea.Batch(status, cmd)
		}
		return m, cmd

	case tagAddedMsg:
		if msg.err != nil {
			return m, m.list.NewStatusMessage(fmt.Sprintf("tag error: %v", msg.err))
		}
		for _, path := range msg.paths {
			m.refreshProjectTags(path)
		}
		if len(msg.paths) > 1 {
			return m, m.list.NewStatusMessage(fmt.Sprintf("tagged %d · %s", len(msg.paths), msg.tag))
		}
		return m, m.list.NewStatusMessage(fmt.Sprintf("tagged · %s", msg.tag))

	case clipboardMsg:
		return m, m.clipboardStatus(msg)

	case annotationSetMsg:
		return m, m.handleAnnotationSet(msg)

	case tea.WindowSizeMsg:
		h, v := 2, 4
		m.width, m.height = msg.Width, msg.Height
		m.resizeList(msg.Width-h, msg.Height-v)
		m.advFilter.width = msg.Width - h
		m.advFilter.height = msg.Height - v
		m.tagOrder.setSize(msg.Width-h, msg.Height-v)
		m.annotate.setSize(msg.Width-h, msg.Height-v)
		m.help.setSize(msg.Width-h, msg.Height-v)
		return m, nil

	case tea.MouseMsg:
		if cmd, handled := m.handleMouseWheel(msg); handled {
			return m, cmd
		}
		if m, cmd, ok := m.handleMouse(msg); ok {
			return m, cmd
		}

	case tea.KeyMsg:
		if cmd, handled := m.handleAppKeys(msg); handled {
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.listDirty && !m.restoringSession {
		m.publishListNow()
	}
	m.refreshTitle()
	m.persistSession()
	return m, cmd
}

func (m *model) noteHelperResult(path string) {
	if m.refresh.done(path) {
		m.publishList()
	}
}

// handleEngineEvent folds an engine event into the local project mirror and
// keeps listening for the next event. Overlay screens route here too.
func (m *model) handleEngineEvent(ev engine.Event) tea.Cmd {
	switch e := ev.(type) {
	case engine.ProjectFound:
		m.upsertProject(projectFromEngine(e.Project))
		m.found = len(m.all)
		m.refreshTitle()
	case engine.ProjectUpdated:
		m.upsertProject(projectFromEngine(e.Project))
		m.noteHelperResult(e.Project.Path)
		m.refreshTitle()
	case engine.ProjectRemoved:
		kept := m.all[:0]
		for _, p := range m.all {
			if p.path != e.Path {
				kept = append(kept, p)
			}
		}
		m.all = kept
		m.found = len(m.all)
		m.publishList()
		m.refreshTitle()
	case engine.ScanStarted:
		m.scanning = true
		m.refreshTitle()
	case engine.ScanFinished:
		m.finishScan(scanFinishedMsg{})
		if e.Err != "" {
			m.err = fmt.Errorf("%s", e.Err)
		}
	case engine.GitHubPending:
		m.refreshTitle()
	case engine.PrefsChanged:
		m.publishList()
		m.refreshTitle()
	}
	return waitEvent(m.events)
}

func (m *model) startRefresh(item projectItem) tea.Cmd {
	return m.startRefreshItems([]projectItem{item})
}

func (m *model) startRefreshItems(items []projectItem) tea.Cmd {
	if len(items) == 0 || m.eng == nil {
		return nil
	}
	paths := make([]string, len(items))
	for i, item := range items {
		paths[i] = item.path
		m.refresh.begin(item.path, m.eng.HelperCount())
	}
	_ = m.eng.Send(context.Background(), engine.RefreshRequest{Paths: paths})
	m.publishList()
	if len(items) == 1 {
		return m.list.NewStatusMessage("refreshing · " + items[0].rel)
	}
	return m.list.NewStatusMessage(fmt.Sprintf("refreshing · %d projects", len(items)))
}

func (m *model) upsertProject(p projectItem) {
	if m.upsertProjectData(p) {
		m.publishList()
	}
}

func (m *model) upsertProjectData(p projectItem) bool {
	if p.meta == nil {
		p.meta = map[string]any{}
	}
	if p.annotations == nil {
		p.annotations = m.store.AnnotationsFor(p.path)
	}
	for i := range m.all {
		if m.all[i].path != p.path {
			continue
		}
		p.tags = mergeTags(m.all[i].tags, p.tags)
		p.meta = mergeMeta(m.all[i].meta, p.meta)
		m.all[i] = p
		return true
	}
	m.all = append(m.all, p)
	return true
}

// refreshProjectAnnotations reloads one project's annotations from the store
// and republishes the list so active view filters re-evaluate.
func (m *model) refreshProjectAnnotations(path string) {
	for i := range m.all {
		if m.all[i].path != path {
			continue
		}
		m.all[i].annotations = m.store.AnnotationsFor(path)
		m.publishList()
		return
	}
}

func (m *model) handleAnnotationSet(msg annotationSetMsg) tea.Cmd {
	if msg.err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("annotation error: %v", msg.err))
	}

	prevIdx := m.list.Index()
	removed := 0
	for _, path := range msg.paths {
		m.refreshProjectAnnotations(path)
		if !m.projectVisibleInView(path) {
			removed++
		}
	}
	m.refreshTitle()

	if m.details.open {
		for _, path := range msg.paths {
			if m.details.path == path && !m.projectVisibleInView(path) {
				items := m.list.VisibleItems()
				if len(items) == 0 {
					m.details.open = false
				} else {
					next := prevIdx
					if next >= len(items) {
						next = len(items) - 1
					}
					if next < 0 {
						next = 0
					}
					if p, ok := items[next].(projectItem); ok {
						m.details.path = p.path
					}
				}
				break
			}
		}
	}

	var status string
	if msg.value == "" {
		status = fmt.Sprintf("cleared @%s", msg.key)
	} else {
		status = fmt.Sprintf("@%s = %s", msg.key, msg.value)
	}
	if len(msg.paths) > 1 {
		status = fmt.Sprintf("%s · %d projects", status, len(msg.paths))
	}
	if removed > 0 {
		if removed == 1 {
			status += " · 1 removed from view"
		} else {
			status += fmt.Sprintf(" · %d removed from view", removed)
		}
	}
	return m.list.NewStatusMessage(status)
}

func (m *model) applyMeta(path, helper string, meta map[string]any) {
	// Kept only for tests that manipulate meta directly; live enrichment
	// arrives as engine.ProjectUpdated events.
	tags, rest := helpers.SplitTags(meta)
	for _, tag := range tags {
		_, _ = m.store.AddTag(path, tag)
	}
	for i := range m.all {
		if m.all[i].path != path {
			continue
		}
		m.all[i].meta = mergeMeta(m.all[i].meta, rest)
		if len(tags) > 0 {
			m.all[i].tags = mergeTags(m.all[i].tags, m.store.TagsFor(path))
		}
		m.markListDirty()
		return
	}
}

func (m *model) markListDirty() {
	m.listDirty = true
}

func (m *model) publishList() {
	if m.scanning {
		m.listDirty = true
		return
	}
	m.publishListNow()
}

func (m *model) publishListNow() {
	m.listDirty = false
	selectedPath := ""
	prevIdx := m.list.Index()
	if cur, ok := m.list.SelectedItem().(projectItem); ok {
		selectedPath = cur.path
	}

	if m.pruneMissingProjects() {
		m.refreshTitle()
		if cur, ok := m.list.SelectedItem().(projectItem); ok {
			selectedPath = cur.path
		}
	}

	items := make([]list.Item, 0, len(m.all))
	for _, p := range m.all {
		if m.projectMatchesView(p) {
			items = append(items, p)
		}
	}

	sortKeys := m.sortKeyList()
	sort.SliceStable(items, func(i, j int) bool {
		return projectLessBy(items[i].(projectItem), items[j].(projectItem), sortKeys)
	})

	filterCmd := m.list.SetItems(items)
	m.syncListFilterMatches(filterCmd)
	m.recomputeTableColumns()

	if !m.restoringSession {
		if selectedPath != "" && m.projectVisibleInView(selectedPath) {
			m.reselect(selectedPath)
		} else if len(m.list.VisibleItems()) > 0 {
			m.reselectAfterRemoval(prevIdx)
		}
	}

	if !m.restoringSession {
		m.persistSession()
	}
}

func (m *model) syncListFilterMatches(filterCmd tea.Cmd) {
	if filterCmd == nil {
		return
	}
	msg := filterCmd()
	if msg == nil {
		return
	}
	m.list, _ = m.list.Update(msg)
}

func (m *model) reselectAfterRemoval(prevIdx int) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	next := prevIdx
	if next >= len(items) {
		next = len(items) - 1
	}
	if next < 0 {
		next = 0
	}
	m.list.Select(next)
}

func (m *model) reselect(selectedPath string) {
	if selectedPath == "" {
		return
	}
	items := m.list.Items()
	for i, it := range items {
		if pi, ok := it.(projectItem); ok && pi.path == selectedPath {
			m.list.Select(i)
			return
		}
	}
}

func (m *model) refreshProjectTags(path string) {
	tags := m.store.TagsFor(path)
	for i := range m.all {
		if m.all[i].path != path {
			continue
		}
		m.all[i].tags = tags
		m.publishList()
		return
	}
}

func (m *model) persistCache() {
	m.pruneMissingProjects()
	cached := make([]store.CachedProject, 0, len(m.all))
	for _, p := range m.all {
		if !project.IsIndexed(p.path) {
			continue
		}
		cached = append(cached, store.CachedProject{Path: p.path, Rel: p.rel})
	}
	_ = m.store.SetCacheRoot(m.root, cached)
}

// pruneMissingProjects silently drops deleted repos from the in-memory list
// and persistent project metadata. Returns true when anything was removed.
func (m *model) pruneMissingProjects() bool {
	removed := false
	kept := m.all[:0]
	for _, p := range m.all {
		if project.IsIndexed(p.path) {
			kept = append(kept, p)
			continue
		}
		removed = true
		_ = m.store.RemoveProject(p.path)
	}
	if !removed {
		return false
	}
	m.all = kept
	m.found = len(m.all)
	return true
}

func (m *model) finishScan(msg scanFinishedMsg) {
	m.scanning = false
	m.err = msg.err
	m.list.StopSpinner()
	m.pruneMissingProjects()
	m.publishListNow()
	m.refreshTitle()
}

func mergeTags(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, set := range [][]string{a, b} {
		for _, tag := range set {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	return out
}

func mergeMeta(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func projectLess(a, b projectItem) bool {
	return strings.ToLower(a.rel) < strings.ToLower(b.rel)
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.advFilter.open {
		return m.advFilter.View()
	}
	if m.layoutMenu.open {
		return m.layoutMenuView()
	}
	if m.viewMenu.open {
		return m.viewMenuView()
	}
	if m.writeMenu.open {
		return m.writeMenuView()
	}
	if m.sortMenu.open {
		return m.sortMenuView()
	}
	if m.help.open {
		return m.helpView()
	}
	if m.tagOrder.open {
		return m.tagOrder.View()
	}
	if m.annotate.open {
		return m.annotate.View()
	}
	if m.details.open {
		return m.detailsView()
	}
	view := "\n" + m.mainListView()
	if m.tagging {
		view += m.renderTagPrompt()
	}
	return view
}

func newModel(root string, st *store.Store) model {
	eng := engine.New(st, root)
	events := eng.Subscribe()

	layout := parseLayoutMode(st.Layout())
	refresh := newRefreshState()
	bulk := newBulkSelectState()
	table := &tableState{enabled: st.TableMode()}

	var m model
	fields := func() []helpers.Field { return m.helperFields() }
	delegate := newItemDelegate(st, fields, layout, refresh, root, bulk)
	delegate.table = table
	l := list.New(nil, delegate, 0, 0)
	l.Title = "RepoManager"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetShowPagination(true)
	l.SetStatusBarItemName("project in the list", "projects in the list")
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	_ = l.StartSpinner()

	l.KeyMap.CursorUp = key.NewBinding(
		key.WithKeys("k", "up", "ctrl+p"),
		key.WithHelp("k/↑", "up"),
	)
	l.KeyMap.CursorDown = key.NewBinding(
		key.WithKeys("j", "down", "ctrl+n"),
		key.WithHelp("j/↓", "down"),
	)
	l.KeyMap.GoToStart = key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g", "first"),
	)
	l.KeyMap.GoToEnd = key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G", "last"),
	)
	l.KeyMap.Filter = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	)
	l.KeyMap.PrevPage = key.NewBinding(
		key.WithKeys("pgup", "u"),
		key.WithHelp("pgup", "prev page"),
	)
	l.KeyMap.NextPage = key.NewBinding(
		key.WithKeys("pgdown", "d"),
		key.WithHelp("pgdn", "next page"),
	)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
			key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "adv filter")),
			key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "layout")),
			key.NewBinding(key.WithKeys("h", "?"), key.WithHelp("?/h", "help")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
			key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "Sort menu")),
			key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "Table")),
			key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tag")),
			key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "tag order")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "annotate")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "bulk select")),
			key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "views")),
			key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "write")),
			key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "copy path")),
			key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank md")),
			key.NewBinding(key.WithKeys("f5"), key.WithHelp("f5", "refresh")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		}
	}

	ti := textinput.New()
	ti.Placeholder = "tag name"
	ti.CharLimit = 32
	ti.Width = 24

	m = model{
		list:       l,
		eng:        eng,
		store:      st,
		root:       root,
		events:     events,
		scanning:   true,
		tagInput:   ti,
		table:      table,
		advFilter:  newAdvancedFilter(),
		tagOrder:   newTagOrderUI(),
		annotate:   newAnnotateUI(),
		layout:     layout,
		layoutMenu: layoutUI{},
		viewMenu:   newViewUI(),
		writeMenu:  newWriteUI(),
		help:       newHelpUI(),
		refresh:    refresh,
		bulk:       bulk,
	}

	for _, p := range eng.Snapshot() {
		m.upsertProjectData(projectFromEngine(p))
	}
	m.found = len(m.all)

	m.lastSession = st.SessionFor(root)
	m.restoringSession = true
	m.restoreSession()
	m.refreshTitle()

	eng.Start()

	return m
}

// Run starts the interactive list over root and blocks until the user quits.
// It returns the path the user selected, or "" if nothing was chosen.
func Run(root string, st *store.Store) (string, error) {
	m := newModel(root, st)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	result, ok := final.(model)
	if !ok {
		return "", nil
	}
	if result.eng != nil {
		result.eng.Close()
	}
	return result.selected, nil
}
