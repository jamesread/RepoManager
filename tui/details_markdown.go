package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func markdownInline(s string) string {
	if s == "" {
		return "_unset_"
	}
	return "`" + strings.ReplaceAll(s, "`", "\\`") + "`"
}

func (m *model) projectDetailsMarkdown(p projectItem) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", p.rel)
	fmt.Fprintf(&b, "`%s`\n\n", filepath.Clean(p.path))

	b.WriteString("## Tags\n\n")
	tags := m.store.SortTagsByImportance(p.tags)
	if len(tags) == 0 {
		b.WriteString("_none_\n\n")
	} else {
		for _, tag := range tags {
			fmt.Fprintf(&b, "- %s\n", tag)
		}
		b.WriteByte('\n')
	}

	keys := m.store.AnnotationKeys()
	if len(keys) > 0 || len(p.annotations) > 0 {
		b.WriteString("## Annotations\n\n")
		seen := map[string]bool{}
		for _, k := range keys {
			seen[k] = true
			fmt.Fprintf(&b, "- **@%s**: %s\n", k, markdownInline(p.annotations[k]))
		}
		var extra []string
		for k := range p.annotations {
			if !seen[k] && p.annotations[k] != "" {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		for _, k := range extra {
			fmt.Fprintf(&b, "- **@%s**: %s\n", k, markdownInline(p.annotations[k]))
		}
		b.WriteByte('\n')
	}

	b.WriteString("## Metadata\n\n")
	shown := 0
	for _, f := range m.helperFields() {
		if f.ID == "name" || f.ID == "tags" {
			continue
		}
		v, present := p.meta[f.ID]
		if !present {
			continue
		}
		fmt.Fprintf(&b, "- **%s**: %s\n", f.Label, markdownInline(formatDetailValue(v)))
		shown++
	}
	if shown == 0 {
		b.WriteString("_none_\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
