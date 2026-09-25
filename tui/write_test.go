package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamesread/RepoManager/core/helpers"
)

func testWriteModel(t *testing.T, items []list.Item) *model {
	t.Helper()
	st := newTestStore(t)
	m := testMouseModel(t, items, layoutComfy)
	m.store = st
	m.root = t.TempDir()
	attachTestEngine(t, m, st, m.root)
	m.list.SetItems(items)
	return m
}

func TestHandleAppKeysWrite(t *testing.T) {
	m := testWriteModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
	})
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	cmd, handled := m.handleAppKeys(msg)
	if !handled {
		t.Fatal("expected w to open write menu")
	}
	if cmd != nil {
		t.Fatal("expected no command when opening write menu")
	}
	if !m.writeMenu.open {
		t.Fatal("write menu should be open")
	}
}

func TestExportCSVIncludesAllFields(t *testing.T) {
	when := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	m := testWriteModel(t, []list.Item{projectItem{
		path: "/tmp/alpha",
		rel:  "alpha",
		tags: []string{"Git"},
		meta: map[string]any{
			helpers.FieldDirMtime: when,
		},
		annotations: map[string]string{"status": "wip"},
	}})
	if _, err := m.store.AddAnnotationKey("status"); err != nil {
		t.Fatal(err)
	}

	data, err := m.exportCSV(m.exportProjects())
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(string(data)))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	header := strings.Join(rows[0], ",")
	for _, want := range []string{"Path", "Name", "Tags", "Dir mtime", "@status"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header %q missing %q", header, want)
		}
	}
	if !strings.Contains(rows[1][0], "/tmp/alpha") {
		t.Fatalf("row=%v", rows[1])
	}
}

func TestExportJSONRoundTrip(t *testing.T) {
	m := testWriteModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a", tags: []string{"Git"}},
		projectItem{path: "/tmp/b", rel: "b"},
	})
	data, err := m.exportJSON(m.exportProjects())
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Projects) != 2 {
		t.Fatalf("projects=%d", len(payload.Projects))
	}
	if payload.Projects[0]["path"] != "/tmp/a" {
		t.Fatalf("first=%v", payload.Projects[0])
	}
}

func TestWriteCurrentViewDeletesDefaultTempOnCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.json")
	m := testWriteModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
	})
	m.writeMenu.format = writeFormatJSON
	m.writeMenu.defaultPath = path
	if err := m.writeCurrentView(writeFormatJSON, path); err != nil {
		t.Fatal(err)
	}
	m.writeMenu.writtenPath = path
	m.writeMenu.deleteOnExit = true

	removed := ""
	osRemove = func(name string) error {
		removed = name
		return nil
	}
	t.Cleanup(func() { osRemove = os.Remove })

	m.writeMenu.cleanup()
	if removed != path {
		t.Fatalf("removed=%q", removed)
	}
}

func TestWriteMenuCommitUsesSelectedFormat(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "projects.md")
	m := testWriteModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "alpha"},
	})
	m.writeMenu.open = true
	m.writeMenu.stage = writeStagePath
	m.writeMenu.format = writeFormatMarkdown
	m.writeMenu.pathInput.SetValue(out)

	cmd := m.commitWrite(false)
	if cmd == nil {
		t.Fatal("expected status command")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("file missing: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "# alpha") {
		t.Fatalf("body=%q", body)
	}
}

func TestWritePathStageAdvancesToClipboard(t *testing.T) {
	m := testWriteModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
	})
	m.writeMenu.open = true
	m.writeMenu.stage = writeStagePath
	m.writeMenu.pathInput.SetValue("/tmp/out.csv")

	updated, cmd := m.updateWritePath(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(model)
	if cmd != nil {
		t.Fatal("expected no cmd when advancing to clipboard stage")
	}
	if um.writeMenu.stage != writeStageClipboard {
		t.Fatalf("stage=%v", um.writeMenu.stage)
	}
}

func TestWriteKeyBinding(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	if key.Matches(msg, key.NewBinding(key.WithKeys("w"))) {
		return
	}
	t.Fatal("w should match write binding")
}
