package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type writeFormat int

const (
	writeFormatMarkdown writeFormat = iota
	writeFormatCSV
	writeFormatJSON
)

type writeFormatEntry struct {
	format writeFormat
	title  string
	desc   string
	ext    string
}

var writeFormatEntries = []writeFormatEntry{
	{writeFormatMarkdown, "Markdown", "one section per project with tags, annotations, and metadata", ".md"},
	{writeFormatCSV, "CSV", "spreadsheet-friendly table with all fields as columns", ".csv"},
	{writeFormatJSON, "JSON", "machine-readable array of project records", ".json"},
}

type exportColumn struct {
	id    string
	label string
}

func (m *model) exportProjects() []projectItem {
	items := m.list.VisibleItems()
	out := make([]projectItem, 0, len(items))
	for _, it := range items {
		if p, ok := it.(projectItem); ok {
			out = append(out, p)
		}
	}
	return out
}

func (m *model) exportColumns() []exportColumn {
	cols := []exportColumn{{id: "path", label: "Path"}}
	for _, f := range m.allFields() {
		cols = append(cols, exportColumn{id: f.ID, label: f.Label})
	}
	return cols
}

func (m *model) exportFieldValue(p projectItem, id string) any {
	if id == "path" {
		return filepath.Clean(p.path)
	}
	return p.fieldValue(id)
}

func formatExportCSVValue(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	case []string:
		return strings.Join(t, "; ")
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", v)
	}
}

func exportJSONValue(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return nil
		}
		return t.UTC().Format(time.RFC3339)
	case []string:
		if len(t) == 0 {
			return nil
		}
		cp := append([]string{}, t...)
		return cp
	default:
		return v
	}
}

func (m *model) exportProjectRecord(p projectItem) map[string]any {
	rec := make(map[string]any, len(m.exportColumns()))
	for _, col := range m.exportColumns() {
		rec[col.id] = exportJSONValue(m.exportFieldValue(p, col.id))
	}
	return rec
}

func (m *model) exportMarkdown(projects []projectItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Project export\n\n")
	if view := m.activeViewTitle(); view != "" {
		fmt.Fprintf(&b, "- **View**: %s\n", view)
	}
	fmt.Fprintf(&b, "- **Sort**: %s\n", m.currentSort().Label)
	fmt.Fprintf(&b, "- **Projects**: %d\n\n", len(projects))
	if len(projects) == 0 {
		b.WriteString("_no projects in current view_\n")
		return b.String()
	}
	for i, p := range projects {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		b.WriteString(m.projectDetailsMarkdown(p))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (m *model) exportCSV(projects []projectItem) ([]byte, error) {
	cols := m.exportColumns()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := make([]string, len(cols))
	for i, col := range cols {
		header[i] = col.label
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, p := range projects {
		row := make([]string, len(cols))
		for i, col := range cols {
			row[i] = formatExportCSVValue(m.exportFieldValue(p, col.id))
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *model) exportJSON(projects []projectItem) ([]byte, error) {
	records := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		records = append(records, m.exportProjectRecord(p))
	}
	payload := map[string]any{
		"scan_root": m.root,
		"view":      m.activeViewTitle(),
		"sort":      m.currentSort().Label,
		"projects":  records,
	}
	return json.MarshalIndent(payload, "", "  ")
}

func (m *model) writeCurrentView(format writeFormat, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	projects := m.exportProjects()
	switch format {
	case writeFormatMarkdown:
		return os.WriteFile(path, []byte(m.exportMarkdown(projects)), 0o644)
	case writeFormatCSV:
		data, err := m.exportCSV(projects)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	case writeFormatJSON:
		data, err := m.exportJSON(projects)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	default:
		return fmt.Errorf("unknown format")
	}
}

func defaultWritePath(format writeFormat) (string, error) {
	ext := ".txt"
	for _, entry := range writeFormatEntries {
		if entry.format == format {
			ext = entry.ext
			break
		}
	}
	f, err := os.CreateTemp("", "plist-export-*"+ext)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}
