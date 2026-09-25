package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func TestEmbedHelpKeyInDesc(t *testing.T) {
	cases := []struct {
		key, desc         string
		prefix, highlight string
		suffix            string
	}{
		{"F", "adv filter", "adv ", "F", "ilter"},
		{"P", "copy path", "copy ", "P", "ath"},
		{"s", "sort", "", "s", "ort"},
		{"O", "tag order", "tag ", "O", "rder"},
		{"a", "annotate", "", "a", "nnotate"},
		{"?/h", "help", "", "h", "elp"},
	}
	for _, tc := range cases {
		prefix, highlight, suffix, ok := embedHelpKeyInDesc(tc.key, tc.desc)
		if !ok {
			t.Fatalf("%q|%q: expected match", tc.key, tc.desc)
		}
		if prefix != tc.prefix || highlight != tc.highlight || suffix != tc.suffix {
			t.Fatalf("%q|%q => (%q,%q,%q) want (%q,%q,%q)",
				tc.key, tc.desc, prefix, highlight, suffix, tc.prefix, tc.highlight, tc.suffix)
		}
	}
}

func TestStyledShortHelpItemEnter(t *testing.T) {
	h := help.New()
	kb := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details"))
	item := styledShortHelpItem(kb, h.Styles)
	if !strings.Contains(item, helpEnterSymbol) {
		t.Fatalf("expected enter symbol, got %q", item)
	}
	if strings.Contains(item, "enter") {
		t.Fatalf("should not show enter text, got %q", item)
	}
	if !strings.Contains(item, "details") {
		t.Fatalf("missing desc: %q", item)
	}
}

func TestStyledShortHelpItemEmbeddedKey(t *testing.T) {
	h := help.New()
	kb := key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "adv filter"))
	item := styledShortHelpItem(kb, h.Styles)
	plain := lipgloss.NewStyle().Render(item)
	if plain != "adv Filter" && plain != "adv filter" {
		// styled output strips to plain text content
		if !strings.Contains(plain, "adv") || !strings.Contains(plain, "ilter") {
			t.Fatalf("got %q", plain)
		}
	}
	if strings.HasPrefix(plain, "F") {
		t.Fatalf("key should be embedded, not leading: %q", plain)
	}
}

func TestComposeTitleBarViewLabel(t *testing.T) {
	st := newTestStore(t)

	m := model{
		width:        120,
		root:         "/tmp/sandbox",
		store:        st,
		activeViewID: viewIDPrefixPreset + "git",
		list:         listForTitleTest(t),
	}
	m.refreshTitle()

	bar := m.composeTitleBar(m.list.Styles.TitleBar.Render(titleStyle.Render(m.titleLeft)))
	if !strings.Contains(bar, "view: Git repos") {
		t.Fatalf("title=%q", bar)
	}

	plain := lipgloss.NewStyle().Render(bar)
	viewIdx := strings.Index(plain, "view: Git repos")
	sortIdx := strings.Index(plain, "sort ")
	if viewIdx < 0 || sortIdx < 0 || viewIdx <= sortIdx {
		t.Fatalf("view should be right of sort: %q", plain)
	}
}

func listForTitleTest(t *testing.T) list.Model {
	t.Helper()
	l := list.New(nil, list.NewDefaultDelegate(), 120, 24)
	l.Title = "RepoManager"
	l.Styles.Title = titleStyle
	return l
}
