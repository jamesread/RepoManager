package main

import (
	"fmt"

	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

// Sorting is driven by an ordered list of sort specs (m.sortSpecs). The first
// entry is the primary key; later entries break ties in priority order. Each
// spec carries its own direction. Name (ascending) is always the final
// tie-breaker, and projects without a value for a field always sort last.

// sortKey is a resolved store.SortSpec.
type sortKey struct {
	Field helpers.Field
	Desc  bool
}

const (
	sortGlyphAsc  = "▴"
	sortGlyphDesc = "▾"
)

func (k sortKey) glyph() string {
	if k.Desc {
		return sortGlyphDesc
	}
	return sortGlyphAsc
}

// directionLabel describes the direction in field-appropriate terms.
func (k sortKey) directionLabel() string {
	switch k.Field.Kind {
	case helpers.FieldTime:
		if k.Desc {
			return "newest first"
		}
		return "oldest first"
	case helpers.FieldInt:
		if k.Desc {
			return "high to low"
		}
		return "low to high"
	default:
		if k.Desc {
			return "Z to A"
		}
		return "A to Z"
	}
}

// defaultSortDesc picks the direction used when a field is first added to
// the sort: times are usually wanted newest first.
func defaultSortDesc(f helpers.Field) bool {
	return f.Kind == helpers.FieldTime
}

var defaultSortField = helpers.Field{ID: "name", Label: "Name", Kind: helpers.FieldString, Sortable: true}

func dedupeSpecs(in []store.SortSpec) []store.SortSpec {
	seen := make(map[string]struct{}, len(in))
	out := make([]store.SortSpec, 0, len(in))
	for _, s := range in {
		if s.ID == "" {
			continue
		}
		if _, ok := seen[s.ID]; ok {
			continue
		}
		seen[s.ID] = struct{}{}
		out = append(out, s)
	}
	return out
}

func specsFromIDs(ids []string) []store.SortSpec {
	out := make([]store.SortSpec, 0, len(ids))
	for _, id := range ids {
		out = append(out, store.SortSpec{ID: id})
	}
	return out
}

// sortSpecsFromPersisted prefers the full specs, then the ordered ID list,
// then the legacy single field.
func sortSpecsFromPersisted(specs []store.SortSpec, ids []string, single string) []store.SortSpec {
	if len(specs) > 0 {
		return append([]store.SortSpec{}, specs...)
	}
	if len(ids) > 0 {
		return specsFromIDs(ids)
	}
	if single != "" {
		return []store.SortSpec{{ID: single}}
	}
	return nil
}

func (m *model) sortableByID() map[string]helpers.Field {
	byID := map[string]helpers.Field{}
	for _, f := range m.sortableFields() {
		byID[f.ID] = f
	}
	return byID
}

// resolvedSortKeys maps m.sortSpecs to known sortable Fields, dropping any
// IDs that no longer exist. It may be empty.
func (m *model) resolvedSortKeys() []sortKey {
	byID := m.sortableByID()
	var out []sortKey
	for _, s := range dedupeSpecs(m.sortSpecs) {
		if f, ok := byID[s.ID]; ok {
			out = append(out, sortKey{Field: f, Desc: s.Desc})
		}
	}
	return out
}

// sortKeyList is resolvedSortKeys with a guaranteed primary (Name ascending).
func (m *model) sortKeyList() []sortKey {
	if out := m.resolvedSortKeys(); len(out) > 0 {
		return out
	}
	for _, f := range m.sortableFields() {
		if f.ID == "name" {
			return []sortKey{{Field: f}}
		}
	}
	return []sortKey{{Field: defaultSortField}}
}

func (m *model) currentSort() helpers.Field {
	return m.sortKeyList()[0].Field
}

func (m *model) currentSortKey() sortKey {
	return m.sortKeyList()[0]
}

// sortFieldIDs returns the effective sort priority as IDs.
func (m *model) sortFieldIDs() []string {
	keys := m.sortKeyList()
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.Field.ID)
	}
	return out
}

// effectiveSortSpecs returns the effective sort priority with directions.
func (m *model) effectiveSortSpecs() []store.SortSpec {
	keys := m.sortKeyList()
	out := make([]store.SortSpec, 0, len(keys))
	for _, k := range keys {
		out = append(out, store.SortSpec{ID: k.Field.ID, Desc: k.Desc})
	}
	return out
}

func (m *model) setSortSpecs(specs []store.SortSpec) {
	m.sortSpecs = dedupeSpecs(specs)
}

// setSortFields sets the priority with every field ascending.
func (m *model) setSortFields(ids []string) {
	m.setSortSpecs(specsFromIDs(ids))
}

func (m *model) sortSpecIndex(id string) int {
	for i, s := range m.sortSpecs {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// setPrimarySort moves id to the front of the sort priority, keeping its
// direction if already present, otherwise using the field default.
func (m *model) setPrimarySort(id string) {
	if id == "" {
		return
	}
	spec := store.SortSpec{ID: id}
	if i := m.sortSpecIndex(id); i >= 0 {
		spec = m.sortSpecs[i]
	} else if f, ok := m.sortableByID()[id]; ok {
		spec.Desc = defaultSortDesc(f)
	}
	out := []store.SortSpec{spec}
	for _, s := range m.sortSpecs {
		if s.ID != id {
			out = append(out, s)
		}
	}
	m.sortSpecs = out
}

// toggleSortField adds id to the end of the priority list, or removes it.
func (m *model) toggleSortField(id string) {
	if i := m.sortSpecIndex(id); i >= 0 {
		m.sortSpecs = append(m.sortSpecs[:i:i], m.sortSpecs[i+1:]...)
		return
	}
	spec := store.SortSpec{ID: id}
	if f, ok := m.sortableByID()[id]; ok {
		spec.Desc = defaultSortDesc(f)
	}
	m.sortSpecs = append(m.sortSpecs, spec)
}

// toggleSortDirection flips the direction of a selected field. Returns false
// when the field is not part of the sort.
func (m *model) toggleSortDirection(id string) bool {
	i := m.sortSpecIndex(id)
	if i < 0 {
		return false
	}
	m.sortSpecs[i].Desc = !m.sortSpecs[i].Desc
	return true
}

// moveSortField shifts id by delta positions within the priority list.
func (m *model) moveSortField(id string, delta int) bool {
	i := m.sortSpecIndex(id)
	if i < 0 {
		return false
	}
	j := i + delta
	if j < 0 || j >= len(m.sortSpecs) || j == i {
		return false
	}
	m.sortSpecs[i], m.sortSpecs[j] = m.sortSpecs[j], m.sortSpecs[i]
	return true
}

// cycleSort advances the primary sort field through every sortable field,
// keeping any secondary keys.
func (m *model) cycleSort() {
	fields := m.sortableFields()
	if len(fields) == 0 {
		return
	}
	cur := m.currentSort().ID
	next := 0
	for i, f := range fields {
		if f.ID == cur {
			next = (i + 1) % len(fields)
			break
		}
	}
	rest := []store.SortSpec{}
	if keys := m.resolvedSortKeys(); len(keys) > 1 {
		for _, k := range keys[1:] {
			rest = append(rest, store.SortSpec{ID: k.Field.ID, Desc: k.Desc})
		}
	}
	primary := store.SortSpec{ID: fields[next].ID, Desc: defaultSortDesc(fields[next])}
	m.setSortSpecs(append([]store.SortSpec{primary}, rest...))
	m.publishList()
	m.refreshTitle()
	m.persistSession()
}

// sortTitleLabel is the compact sort description used in the title bar.
func (m *model) sortTitleLabel() string {
	keys := m.sortKeyList()
	label := keys[0].Field.Label + keys[0].glyph()
	if len(keys) > 1 {
		label += fmt.Sprintf(" +%d", len(keys)-1)
	}
	return label
}

// compareSortKey orders a and b by one key: -1 when a sorts first, 1 when b
// does, 0 when tied. Missing values always sort last.
func compareSortKey(a, b projectItem, k sortKey) int {
	av, bv := a.fieldValue(k.Field.ID), b.fieldValue(k.Field.ID)
	aMissing, bMissing := av == nil, bv == nil
	switch {
	case aMissing && bMissing:
		return 0
	case aMissing:
		return 1
	case bMissing:
		return -1
	}
	cmp := helpers.CompareMeta(k.Field.Kind, av, bv)
	if k.Field.Kind == helpers.FieldTime {
		// helpers.CompareMeta puts newer first; ascending means oldest first.
		cmp = -cmp
	}
	if k.Desc {
		cmp = -cmp
	}
	return cmp
}

// projectLessBy orders two projects by the given sort keys, falling back to
// name ascending.
func projectLessBy(a, b projectItem, keys []sortKey) bool {
	for _, k := range keys {
		if cmp := compareSortKey(a, b, k); cmp != 0 {
			return cmp < 0
		}
	}
	return projectLess(a, b)
}
