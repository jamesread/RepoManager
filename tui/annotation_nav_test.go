package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestListSuccessorPath(t *testing.T) {
	m := &model{
		list: list.New(nil, list.NewDefaultDelegate(), 80, 24),
	}
	m.list.SetItems([]list.Item{
		projectItem{path: "/a", rel: "a"},
		projectItem{path: "/b", rel: "b"},
		projectItem{path: "/c", rel: "c"},
	})

	if got := m.listSuccessorPath("/a"); got != "/b" {
		t.Fatalf("successor(/a)=%q, want /b", got)
	}
	if got := m.listSuccessorPath("/b"); got != "/c" {
		t.Fatalf("successor(/b)=%q, want /c", got)
	}
	if got := m.listSuccessorPath("/c"); got != "" {
		t.Fatalf("successor(/c)=%q, want empty", got)
	}
	if got := m.listSuccessorPath("/missing"); got != "" {
		t.Fatalf("successor(/missing)=%q, want empty", got)
	}
}
