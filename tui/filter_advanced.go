package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/helpers"
)

type filterRule struct {
	FieldID string
	Label   string
	Value   string
	// Values matches multi-value fields (e.g. tags) when the project has all
	// listed values. Value is used for single-value fields.
	Values []string
	// Negated excludes projects that have any of these values (multi fields).
	Negated []string
	// Unset matches projects where the field has no value (null/empty).
	Unset bool
}

type filterValueState int

const (
	filterValueIgnore filterValueState = iota
	filterValueMatch
	filterValueNegate
)

func (s filterValueState) next() filterValueState {
	return (s + 1) % 3
}

func filterValueMark(state filterValueState) string {
	switch state {
	case filterValueMatch:
		return "[+]"
	case filterValueNegate:
		return "[-]"
	default:
		return "[ ]"
	}
}

func (r filterRule) describe() string {
	if r.Unset {
		return r.Label + " is unset"
	}
	pos := r.activeValues()
	neg := r.activeNegated()
	var parts []string
	if len(pos) > 0 {
		if len(pos) > 1 {
			parts = append(parts, fmt.Sprintf("%s=%s", r.Label, strings.Join(pos, "+")))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", r.Label, pos[0]))
		}
	}
	if len(neg) > 0 {
		parts = append(parts, fmt.Sprintf("!%s=%s", r.Label, strings.Join(neg, "+")))
	}
	if len(parts) == 0 {
		return r.Label + " is unset"
	}
	return strings.Join(parts, " · ")
}

func (r filterRule) activeValues() []string {
	if len(r.Values) > 0 {
		return append([]string{}, r.Values...)
	}
	if v := strings.TrimSpace(r.Value); v != "" {
		return []string{v}
	}
	return nil
}

func (r filterRule) activeNegated() []string {
	if len(r.Negated) > 0 {
		return append([]string{}, r.Negated...)
	}
	return nil
}

const (
	filterChoiceClearAll   = "__clear__"
	filterChoiceClearField = "__clear_field__"
	filterChoiceUnset      = "__unset__"
	filterChoiceApplyMulti = "__apply_multi__"
)

type filterStage int

const (
	filterStageFields filterStage = iota
	filterStageValues
)

type filterChoiceItem struct {
	title string
	desc  string
	id    string // field id or special action
	value string // value when picking values
}

func (i filterChoiceItem) Title() string       { return i.title }
func (i filterChoiceItem) Description() string { return i.desc }
func (i filterChoiceItem) FilterValue() string { return i.title + " " + i.desc }

type advancedFilter struct {
	open        bool
	stage       filterStage
	list        list.Model
	field       helpers.Field
	rules       []filterRule
	multiSelect bool
	valueStates map[string]filterValueState
	width       int
	height      int
}

func newAdvancedFilter() advancedFilter {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Advanced Filter"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down"))
	return advancedFilter{list: l}
}

func (a advancedFilter) activeSummary() string {
	if len(a.rules) == 0 {
		return ""
	}
	parts := make([]string, 0, len(a.rules))
	for _, r := range a.rules {
		parts = append(parts, r.describe())
	}
	return strings.Join(parts, " · ")
}

func (m *model) openAdvancedFilter() {
	m.advFilter.open = true
	m.advFilter.stage = filterStageFields
	m.populateFilterFields()
}

func (m *model) populateFilterFields() {
	fields := m.filterableFields()
	items := make([]list.Item, 0, len(fields)+1)
	if len(m.advFilter.rules) > 0 {
		items = append(items, filterChoiceItem{
			title: "Clear all filters",
			desc:  m.advFilter.activeSummary(),
			id:    filterChoiceClearAll,
		})
	}
	for _, f := range fields {
		desc := string(f.Kind)
		if _, isAnnotation := annotationKeyFromField(f.ID); isAnnotation {
			desc = "annotation"
		}
		for _, r := range m.advFilter.rules {
			if r.FieldID == f.ID {
				desc = "active: " + r.describe()
				break
			}
		}
		items = append(items, filterChoiceItem{
			title: f.Label,
			desc:  desc,
			id:    f.ID,
		})
	}
	m.advFilter.list.Title = "Advanced Filter · fields"
	m.advFilter.list.SetItems(items)
	if m.advFilter.width > 0 {
		m.advFilter.list.SetSize(m.advFilter.width, m.advFilter.height)
	}
}

func (m *model) populateFilterValues(field helpers.Field) {
	m.advFilter.field = field
	m.advFilter.stage = filterStageValues
	m.advFilter.multiSelect = field.Kind == helpers.FieldMulti
	m.advFilter.valueStates = map[string]filterValueState{}
	for _, r := range m.advFilter.rules {
		if r.FieldID != field.ID || r.Unset {
			continue
		}
		for _, v := range r.Values {
			m.advFilter.valueStates[v] = filterValueMatch
		}
		for _, v := range r.Negated {
			m.advFilter.valueStates[v] = filterValueNegate
		}
		break
	}

	uniq := map[string]struct{}{}
	unsetCount := 0
	for _, p := range m.all {
		raw := p.fieldValue(field.ID)
		vals := helpers.MetaValuesForBrowse(field, raw)
		if len(vals) == 0 {
			unsetCount++
			continue
		}
		for _, v := range vals {
			uniq[v] = struct{}{}
		}
	}

	values := make([]string, 0, len(uniq))
	for v := range uniq {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool {
		return strings.ToLower(values[i]) < strings.ToLower(values[j])
	})

	items := make([]list.Item, 0, len(values)+3)
	items = append(items, filterChoiceItem{
		title: "(clear this field)",
		desc:  "remove filter on " + field.Label,
		id:    filterChoiceClearField,
		value: "",
	})
	items = append(items, filterChoiceItem{
		title: "(unset)",
		desc:  fmt.Sprintf("%d project(s) with no %s", unsetCount, field.Label),
		id:    filterChoiceUnset,
		value: "",
	})
	if m.advFilter.multiSelect {
		items = append(items, filterChoiceItem{
			title: "(apply filter)",
			desc:  m.filterApplyDescription(),
			id:    filterChoiceApplyMulti,
			value: "",
		})
	}
	for _, v := range values {
		title := v
		if m.advFilter.multiSelect {
			title = filterValueMark(m.advFilter.valueState(v)) + " " + v
		}
		items = append(items, filterChoiceItem{
			title: title,
			desc:  field.Label,
			id:    field.ID,
			value: v,
		})
	}
	m.advFilter.list.Title = "Advanced Filter · " + field.Label
	if m.advFilter.multiSelect {
		m.advFilter.list.Title += " · multi"
	}
	m.advFilter.list.SetItems(items)
	if m.advFilter.width > 0 {
		m.advFilter.list.SetSize(m.advFilter.width, m.advFilter.height)
	}
}

func (a *advancedFilter) valueState(value string) filterValueState {
	if a.valueStates == nil {
		return filterValueIgnore
	}
	return a.valueStates[value]
}

func (m *model) filterMatchedValues() []string {
	return m.filterValuesByState(filterValueMatch)
}

func (m *model) filterNegatedValues() []string {
	return m.filterValuesByState(filterValueNegate)
}

func (m *model) filterValuesByState(want filterValueState) []string {
	out := make([]string, 0, len(m.advFilter.valueStates))
	for v, s := range m.advFilter.valueStates {
		if s == want {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func (m *model) filterApplyDescription() string {
	match := m.filterMatchedValues()
	neg := m.filterNegatedValues()
	if len(match) == 0 && len(neg) == 0 {
		return "no values active · clears filter on apply"
	}
	var parts []string
	if len(match) > 0 {
		parts = append(parts, "match "+strings.Join(match, "+"))
	}
	if len(neg) > 0 {
		parts = append(parts, "exclude "+strings.Join(neg, "+"))
	}
	return strings.Join(parts, " · ")
}

func (m *model) cycleFilterValue(value string) {
	if value == "" {
		return
	}
	if m.advFilter.valueStates == nil {
		m.advFilter.valueStates = map[string]filterValueState{}
	}
	next := m.advFilter.valueState(value).next()
	if next == filterValueIgnore {
		delete(m.advFilter.valueStates, value)
	} else {
		m.advFilter.valueStates[value] = next
	}
	m.refreshFilterValueItems()
}

func (m *model) refreshFilterValueItems() {
	field := m.advFilter.field
	cursor := m.advFilter.list.Index()

	uniq := map[string]struct{}{}
	unsetCount := 0
	for _, p := range m.all {
		raw := p.fieldValue(field.ID)
		vals := helpers.MetaValuesForBrowse(field, raw)
		if len(vals) == 0 {
			unsetCount++
			continue
		}
		for _, v := range vals {
			uniq[v] = struct{}{}
		}
	}

	values := make([]string, 0, len(uniq))
	for v := range uniq {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool {
		return strings.ToLower(values[i]) < strings.ToLower(values[j])
	})

	items := make([]list.Item, 0, len(values)+3)
	items = append(items, filterChoiceItem{
		title: "(clear this field)",
		desc:  "remove filter on " + field.Label,
		id:    filterChoiceClearField,
		value: "",
	})
	items = append(items, filterChoiceItem{
		title: "(unset)",
		desc:  fmt.Sprintf("%d project(s) with no %s", unsetCount, field.Label),
		id:    filterChoiceUnset,
		value: "",
	})
	if m.advFilter.multiSelect {
		items = append(items, filterChoiceItem{
			title: "(apply filter)",
			desc:  m.filterApplyDescription(),
			id:    filterChoiceApplyMulti,
			value: "",
		})
	}
	for _, v := range values {
		title := v
		if m.advFilter.multiSelect {
			title = filterValueMark(m.advFilter.valueState(v)) + " " + v
		}
		items = append(items, filterChoiceItem{
			title: title,
			desc:  field.Label,
			id:    field.ID,
			value: v,
		})
	}
	m.advFilter.list.SetItems(items)
	if cursor >= 0 && cursor < len(items) {
		m.advFilter.list.Select(cursor)
	}
}

func (m model) updateAdvancedFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.advFilter.width = msg.Width - 2
		m.advFilter.height = msg.Height - 4
		m.advFilter.list.SetSize(m.advFilter.width, m.advFilter.height)
		m.resizeList(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.advFilter.stage == filterStageValues {
				m.advFilter.stage = filterStageFields
				m.advFilter.multiSelect = false
				m.advFilter.valueStates = nil
				m.populateFilterFields()
				return m, nil
			}
			m.advFilter.open = false
			return m, nil
		case m.advFilter.stage == filterStageValues && m.advFilter.multiSelect &&
			key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
			item, ok := m.advFilter.list.SelectedItem().(filterChoiceItem)
			if ok && item.id == m.advFilter.field.ID && item.value != "" {
				m.cycleFilterValue(item.value)
				return m, nil
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			item, ok := m.advFilter.list.SelectedItem().(filterChoiceItem)
			if !ok {
				return m, nil
			}
			if m.advFilter.stage == filterStageValues && m.advFilter.multiSelect &&
				item.id == m.advFilter.field.ID && item.value != "" {
				m.cycleFilterValue(item.value)
				return m, nil
			}
			return m.applyFilterChoice(item)
		}
	}

	var cmd tea.Cmd
	m.advFilter.list, cmd = m.advFilter.list.Update(msg)
	return m, cmd
}

func (m model) applyFilterChoice(item filterChoiceItem) (tea.Model, tea.Cmd) {
	switch {
	case item.id == filterChoiceClearAll:
		m.advFilter.rules = nil
		m.advFilter.open = false
		m.advFilter.multiSelect = false
		m.advFilter.valueStates = nil
		m.publishList()
		m.refreshTitle()
		return m, m.list.NewStatusMessage("filters cleared")
	case item.id == filterChoiceClearField:
		m.removeFilterRule(m.advFilter.field.ID)
		m.advFilter.open = false
		m.advFilter.multiSelect = false
		m.advFilter.valueStates = nil
		m.publishList()
		m.refreshTitle()
		return m, m.list.NewStatusMessage("cleared " + m.advFilter.field.Label)
	case item.id == filterChoiceUnset:
		rule := filterRule{
			FieldID: m.advFilter.field.ID,
			Label:   m.advFilter.field.Label,
			Unset:   true,
		}
		m.setFilterRule(rule)
		m.advFilter.open = false
		m.advFilter.multiSelect = false
		m.advFilter.valueStates = nil
		m.publishList()
		m.refreshTitle()
		return m, m.list.NewStatusMessage("filter · " + rule.describe())
	case item.id == filterChoiceApplyMulti:
		return m.applyMultiFilterRule()
	case m.advFilter.stage == filterStageFields:
		field, ok := m.fieldByID(item.id)
		if !ok {
			return m, nil
		}
		m.populateFilterValues(field)
		return m, nil
	default:
		rule := filterRule{
			FieldID: m.advFilter.field.ID,
			Label:   m.advFilter.field.Label,
			Value:   item.value,
		}
		m.setFilterRule(rule)
		m.advFilter.open = false
		m.publishList()
		m.refreshTitle()
		return m, m.list.NewStatusMessage("filter · " + rule.describe())
	}
}

func (m model) applyMultiFilterRule() (tea.Model, tea.Cmd) {
	match := m.filterMatchedValues()
	neg := m.filterNegatedValues()
	field := m.advFilter.field
	if len(match) == 0 && len(neg) == 0 {
		m.removeFilterRule(field.ID)
		m.advFilter.open = false
		m.advFilter.multiSelect = false
		m.advFilter.valueStates = nil
		m.publishList()
		m.refreshTitle()
		return m, m.list.NewStatusMessage("cleared " + field.Label)
	}
	rule := filterRule{
		FieldID: field.ID,
		Label:   field.Label,
		Values:  match,
		Negated: neg,
	}
	m.setFilterRule(rule)
	m.advFilter.open = false
	m.advFilter.multiSelect = false
	m.advFilter.valueStates = nil
	m.publishList()
	m.refreshTitle()
	return m, m.list.NewStatusMessage("filter · " + rule.describe())
}

func (m *model) setFilterRule(rule filterRule) {
	for i, existing := range m.advFilter.rules {
		if existing.FieldID == rule.FieldID {
			m.advFilter.rules[i] = rule
			return
		}
	}
	m.advFilter.rules = append(m.advFilter.rules, rule)
}

func (m *model) removeFilterRule(fieldID string) {
	out := m.advFilter.rules[:0]
	for _, r := range m.advFilter.rules {
		if r.FieldID != fieldID {
			out = append(out, r)
		}
	}
	m.advFilter.rules = out
}

func matchFilterRule(p projectItem, rule filterRule, field helpers.Field) bool {
	raw := p.fieldValue(rule.FieldID)
	vals := helpers.MetaValuesForBrowse(field, raw)

	if rule.Unset {
		return len(vals) == 0
	}

	for _, ban := range rule.activeNegated() {
		for _, v := range vals {
			if strings.EqualFold(v, ban) {
				return false
			}
		}
	}

	if selected := rule.activeValues(); len(selected) > 0 {
		if field.Kind == helpers.FieldMulti {
			for _, want := range selected {
				found := false
				for _, v := range vals {
					if strings.EqualFold(v, want) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
		want := strings.ToLower(strings.TrimSpace(selected[0]))
		for _, v := range vals {
			if strings.EqualFold(v, selected[0]) || strings.Contains(strings.ToLower(v), want) {
				return true
			}
		}
		return false
	}

	if len(rule.activeNegated()) > 0 {
		return true
	}

	return true
}

func (m *model) projectMatchesFilters(p projectItem) bool {
	for _, rule := range m.advFilter.rules {
		field, _ := m.fieldByID(rule.FieldID)
		if !matchFilterRule(p, rule, field) {
			return false
		}
	}
	return true
}

func (m *model) projectMatchesQuickFilter(p projectItem) bool {
	switch m.list.FilterState() {
	case list.Unfiltered, list.Filtering:
		return true
	}
	term := strings.TrimSpace(m.list.FilterValue())
	if term == "" {
		return true
	}
	return len(m.list.Filter(term, []string{p.FilterValue()})) > 0
}

func (m *model) projectMatchesView(p projectItem) bool {
	return m.projectMatchesFilters(p) && m.projectMatchesQuickFilter(p)
}

func (m *model) projectVisibleInView(path string) bool {
	for _, it := range m.list.VisibleItems() {
		if p, ok := it.(projectItem); ok && p.path == path {
			return true
		}
	}
	return false
}

func (a advancedFilter) View() string {
	if !a.open {
		return ""
	}
	help := "enter select · esc back/close · / filter values · (unset) matches empty"
	if a.stage == filterStageValues && a.multiSelect {
		help = "space cycle [ ]/[+]/[-] · enter apply/cycle · esc back/close · / filter values"
	}
	return "\n" + a.list.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help)
}
