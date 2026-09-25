package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamesread/RepoManager/core/helpers"
)

func TestDetailsYankKeyBinding(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	if !key.Matches(msg, detailsYank) {
		t.Fatal("y should match detailsYank binding")
	}
}

func TestHandleAppKeysYank(t *testing.T) {
	m := testMouseModel(t, []list.Item{
		projectItem{path: "/tmp/a", rel: "a"},
	}, layoutComfy)
	m.list.SetItems(m.list.Items())

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	cmd, handled := m.handleAppKeys(msg)
	if !handled {
		t.Fatal("expected y to be handled on main list")
	}
	if cmd == nil {
		t.Fatal("expected yank clipboard command")
	}
}

func TestDetailsViewGitHubURL(t *testing.T) {
	st := newTestStore(t)
	m := model{
		width:   120,
		store:   st,
		details: detailsUI{open: true, path: "/tmp/p"},
		all: []projectItem{{
			path: "/tmp/p",
			rel:  "widgets",
			meta: helpers.GitHubBasicMeta("acme", "widgets"),
		}},
	}
	view := m.detailsView()
	if !strings.Contains(view, "https://github.com/acme/widgets") {
		t.Fatalf("view missing github url:\n%s", view)
	}
}
