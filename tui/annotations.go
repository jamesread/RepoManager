package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jamesread/RepoManager/core/helpers"
)

// Annotations are user-defined key/value pairs. Keys are global (defined
// once in settings); values are per project and optional. They are surfaced
// as synthetic Fields so the sort cycle and Advanced Filter treat them like
// helper metadata.

const annotationFieldPrefix = "annotation."

func annotationFieldID(key string) string {
	return annotationFieldPrefix + key
}

// annotationKeyFromField returns the key for an annotation field id.
func annotationKeyFromField(fieldID string) (string, bool) {
	if !strings.HasPrefix(fieldID, annotationFieldPrefix) {
		return "", false
	}
	key := strings.TrimPrefix(fieldID, annotationFieldPrefix)
	return key, key != ""
}

func annotationField(key string) helpers.Field {
	return helpers.Field{
		ID:         annotationFieldID(key),
		Label:      "@" + key,
		Kind:       helpers.FieldString,
		Sortable:   true,
		Filterable: true,
	}
}

// annotationFields builds one helpers.Field per global annotation key.
func annotationFields(keys []string) []helpers.Field {
	out := make([]helpers.Field, 0, len(keys))
	for _, key := range keys {
		out = append(out, annotationField(key))
	}
	return out
}

// allFields is the complete field catalogue: helper fields plus annotations.
func (m *model) allFields() []helpers.Field {
	fields := m.helperFields()
	return append(fields, annotationFields(m.store.AnnotationKeys())...)
}

func (m *model) filterableFields() []helpers.Field {
	var out []helpers.Field
	for _, f := range m.allFields() {
		if f.Filterable {
			out = append(out, f)
		}
	}
	return out
}

func (m *model) sortableFields() []helpers.Field {
	var out []helpers.Field
	for _, f := range m.allFields() {
		if f.Sortable {
			out = append(out, f)
		}
	}
	return out
}

func (m *model) fieldByID(id string) (helpers.Field, bool) {
	for _, f := range m.helperFields() {
		if f.ID == id {
			return f, true
		}
	}
	if key, ok := annotationKeyFromField(id); ok {
		return annotationField(key), true
	}
	return helpers.Field{}, false
}

// fieldValue resolves any field id against a project: built-ins, annotation
// fields and helper metadata. nil means the value is unset.
func (p projectItem) fieldValue(id string) any {
	switch id {
	case "name":
		return p.rel
	case "tags":
		if len(p.tags) == 0 {
			return nil
		}
		return append([]string{}, p.tags...)
	}
	if key, ok := annotationKeyFromField(id); ok {
		if v, present := p.annotations[key]; present && v != "" {
			return v
		}
		return nil
	}
	return p.metaValue(id)
}

// valueCount pairs an annotation value with its usage count.
type valueCount struct {
	value string
	count int
}

// rankValueCounts orders values by count (desc) then value (asc, case-insensitive).
func rankValueCounts(counts map[string]int) []valueCount {
	out := make([]valueCount, 0, len(counts))
	for v, n := range counts {
		out = append(out, valueCount{value: v, count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		return strings.ToLower(out[i].value) < strings.ToLower(out[j].value)
	})
	return out
}

// filterValueCounts keeps suggestions containing the typed text
// (case-insensitive). Empty text keeps everything.
func filterValueCounts(all []valueCount, typed string) []valueCount {
	typed = strings.ToLower(strings.TrimSpace(typed))
	if typed == "" {
		return all
	}
	out := make([]valueCount, 0, len(all))
	for _, vc := range all {
		if strings.Contains(strings.ToLower(vc.value), typed) {
			out = append(out, vc)
		}
	}
	return out
}

// annotationSummary renders "@key=value" pairs in key definition order.
func annotationSummary(keys []string, values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	ordered := append([]string{}, keys...)
	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		seen[k] = true
	}
	var extra []string
	for k := range values {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	ordered = append(ordered, extra...)

	parts := make([]string, 0, len(values))
	for _, k := range ordered {
		if v, ok := values[k]; ok && v != "" {
			parts = append(parts, fmt.Sprintf("@%s=%s", k, v))
		}
	}
	return strings.Join(parts, " ")
}
