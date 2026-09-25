package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type bulkSelectState struct {
	active   bool
	selected map[string]struct{}
}

// newBulkSelectState returns a pointer so the model and the list delegate
// share one state: the model is copied by value on every Update, and a
// pointer into an old copy would never see later changes to `active`.
func newBulkSelectState() *bulkSelectState {
	return &bulkSelectState{selected: map[string]struct{}{}}
}

// isActive reports whether bulk select is on. Safe on a nil receiver so
// models built without a bulk state (tests) read as "off".
func (b *bulkSelectState) isActive() bool {
	return b != nil && b.active
}

func (b *bulkSelectState) clear() {
	b.selected = map[string]struct{}{}
}

func (b *bulkSelectState) count() int {
	if b == nil {
		return 0
	}
	return len(b.selected)
}

func (b *bulkSelectState) isSelected(path string) bool {
	if b == nil {
		return false
	}
	_, ok := b.selected[path]
	return ok
}

func (b *bulkSelectState) toggle(path string) {
	if path == "" {
		return
	}
	if b.isSelected(path) {
		delete(b.selected, path)
		return
	}
	b.selected[path] = struct{}{}
}

func (b *bulkSelectState) paths() []string {
	out := make([]string, 0, len(b.selected))
	for path := range b.selected {
		out = append(out, path)
	}
	return out
}

func (m *model) projectsByPaths(paths []string) []projectItem {
	if len(paths) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		want[path] = struct{}{}
	}
	out := make([]projectItem, 0, len(paths))
	for _, p := range m.all {
		if _, ok := want[p.path]; ok {
			out = append(out, p)
			delete(want, p.path)
			if len(want) == 0 {
				break
			}
		}
	}
	return out
}

func (m *model) operationTargets() []projectItem {
	if m.bulk.isActive() {
		if paths := m.bulk.paths(); len(paths) > 0 {
			return m.projectsByPaths(paths)
		}
	}
	if item, ok := m.list.SelectedItem().(projectItem); ok {
		return []projectItem{item}
	}
	return nil
}

func (m *model) toggleBulkSelect() tea.Cmd {
	if m.bulk == nil {
		m.bulk = newBulkSelectState()
	}
	if m.bulk.active {
		m.bulk.active = false
		m.bulk.clear()
		m.refreshTitle()
		m.recomputeTableColumns()
		return m.list.NewStatusMessage("bulk select off")
	}
	m.bulk.active = true
	if item, ok := m.list.SelectedItem().(projectItem); ok {
		m.bulk.toggle(item.path)
	}
	m.refreshTitle()
	m.recomputeTableColumns()
	return m.list.NewStatusMessage("bulk select · space toggles · b exits")
}

func (m *model) toggleBulkItem(path string) tea.Cmd {
	m.toggleBulkItemSilent(path)
	return m.list.NewStatusMessage(fmt.Sprintf("bulk · %d selected", m.bulk.count()))
}

func (m *model) toggleBulkItemSilent(path string) {
	if m.bulk == nil {
		return
	}
	m.bulk.toggle(path)
	m.refreshTitle()
}

func (m *model) handleBulkToggleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !key.Matches(msg, key.NewBinding(key.WithKeys(" "))) || !m.bulk.isActive() {
		return nil, false
	}
	item, ok := m.list.SelectedItem().(projectItem)
	if !ok {
		return nil, true
	}
	return m.toggleBulkItem(item.path), true
}
