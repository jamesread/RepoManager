package main

import (
	"reflect"
	"strings"

	"github.com/jamesread/RepoManager/core/store"
)

func persistFilterRules(rules []filterRule) []store.FilterRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]store.FilterRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, store.FilterRule{
			FieldID: r.FieldID,
			Value:   r.Value,
			Values:  append([]string{}, r.Values...),
			Negated: append([]string{}, r.Negated...),
			Unset:   r.Unset,
		})
	}
	return out
}

func restoreFilterRules(m *model, persisted []store.FilterRule) []filterRule {
	if len(persisted) == 0 {
		return nil
	}
	out := make([]filterRule, 0, len(persisted))
	for _, p := range persisted {
		field, ok := m.fieldByID(p.FieldID)
		if !ok {
			continue
		}
		out = append(out, filterRule{
			FieldID: p.FieldID,
			Label:   field.Label,
			Value:   p.Value,
			Values:  append([]string{}, p.Values...),
			Negated: append([]string{}, p.Negated...),
			Unset:   p.Unset,
		})
	}
	return out
}

func (m *model) viewIDExists(id string) bool {
	if _, ok := predefinedViewByID(id); ok {
		return true
	}
	if strings.HasPrefix(id, viewIDPrefixSaved) {
		name := strings.TrimPrefix(id, viewIDPrefixSaved)
		_, ok := m.store.ViewNamed(m.root, name)
		return ok
	}
	return false
}

func (m *model) currentSession() store.RootSession {
	selected := ""
	if cur, ok := m.list.SelectedItem().(projectItem); ok {
		selected = cur.path
	}
	return store.RootSession{
		SelectedPath: selected,
		ListIndex:    m.list.Index(),
		QuickFilter:  strings.TrimSpace(m.list.FilterValue()),
		SortFieldID:  m.currentSort().ID,
		SortFieldIDs: m.sortFieldIDs(),
		SortSpecs:    m.effectiveSortSpecs(),
		AdvFilter:    persistFilterRules(m.advFilter.rules),
		ActiveViewID: m.activeViewID,
	}
}

func (m *model) applySessionState(sess store.RootSession) {
	if specs := sortSpecsFromPersisted(sess.SortSpecs, sess.SortFieldIDs, sess.SortFieldID); len(specs) > 0 {
		m.setSortSpecs(specs)
	}

	m.advFilter.rules = restoreFilterRules(m, sess.AdvFilter)

	if sess.QuickFilter != "" {
		m.list.SetFilterText(sess.QuickFilter)
	} else {
		m.list.ResetFilter()
	}

	m.restoringSession = true
	m.publishListNow()

	if sess.SelectedPath != "" {
		m.reselect(sess.SelectedPath)
	}
	if cur, ok := m.list.SelectedItem().(projectItem); !ok || (sess.SelectedPath != "" && cur.path != sess.SelectedPath) {
		m.reselectByIndex(sess.ListIndex)
	}

	m.restoringSession = false
	m.refreshTitle()
}

func (m *model) restoreSession() {
	sess := m.store.SessionFor(m.root)
	m.lastSession = sess

	activeViewID := sess.ActiveViewID
	if activeViewID != "" && !m.viewIDExists(activeViewID) {
		activeViewID = ""
	}
	m.activeViewID = activeViewID

	m.applySessionState(sess)
	m.lastSession = m.currentSession()
}

func (m *model) persistSession() {
	snap := m.currentSession()
	if reflect.DeepEqual(snap, m.lastSession) {
		return
	}
	m.lastSession = snap
	m.persistSessionSnap(snap)
}

func (m *model) persistSessionForce() {
	snap := m.currentSession()
	m.lastSession = snap
	m.persistSessionSnap(snap)
}

func (m *model) reselectByIndex(idx int) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return
	}
	if idx >= len(items) {
		idx = len(items) - 1
	}
	if idx < 0 {
		idx = 0
	}
	m.list.Select(idx)
}
