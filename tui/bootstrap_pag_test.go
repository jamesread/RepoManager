package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamesread/RepoManager/core/store"
)

func TestBootstrapShowsCachedListImmediately(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	paths := make([]store.CachedProject, 0, 5)
	for i, name := range []string{"a", "b", "c", "d", "e"} {
		p := testGitProjectDir(t, name)
		paths = append(paths, store.CachedProject{Path: p, Rel: name})
		_ = i
	}
	_ = st.SetCacheRoot(root, paths)

	m := newModel(root, st)
	t.Cleanup(func() {
		if m.eng != nil {
			m.eng.Close()
		}
	})
	if len(m.list.Items()) == 0 {
		t.Fatal("list should have cached items before scan finishes")
	}
	if m.scanning && len(m.all) == 0 {
		t.Fatal("all should have cached projects")
	}
}

func TestPaginationInMainListView(t *testing.T) {
	st := newTestStore(t)
	delegate := newItemDelegate(st, nil, layoutComfy, newRefreshState(), t.TempDir(), nil)
	items := make([]list.Item, 30)
	for i := range items {
		items[i] = projectItem{path: "/p", rel: "proj"}
	}
	l := list.New(items, delegate, 80, 40)
	l.SetShowPagination(true)
	m := model{list: l, store: st, layout: layoutComfy, width: 80, height: 40}
	view := m.mainListView()
	if !strings.Contains(view, "page 1 of") || !strings.Contains(view, "30 projects") {
		t.Fatalf("expected pagination summary in view:\n%s", view)
	}
}

func TestPgUpThroughModelUpdate(t *testing.T) {
	st := newTestStore(t)
	root := t.TempDir()
	paths := make([]store.CachedProject, 0, 30)
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("proj%d", i)
		p := testGitProjectDir(t, name)
		paths = append(paths, store.CachedProject{Path: p, Rel: name})
	}
	_ = st.SetCacheRoot(root, paths)

	m := newModel(root, st)
	t.Cleanup(func() {
		if m.eng != nil {
			m.eng.Close()
		}
	})
	m.width, m.height = 80, 40
	m.list.SetSize(80, 40)

	startPage := m.list.Paginator.Page
	if m.list.Paginator.TotalPages < 2 {
		t.Fatalf("expected multiple pages, got %d items per page %d pages %d",
			len(m.list.Items()), m.list.Paginator.PerPage, m.list.Paginator.TotalPages)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m2 := updated.(model)
	if m2.list.Paginator.Page == startPage {
		t.Fatalf("pgdown via model.Update should change page")
	}
	view := m2.mainListView()
	wantPage := fmt.Sprintf("page %d of", m2.list.Paginator.Page+1)
	if !strings.Contains(view, wantPage) {
		t.Fatalf("expected %q in view:\n%s", wantPage, view)
	}
	if !strings.Contains(view, "30 projects") {
		t.Fatalf("expected project total in view:\n%s", view)
	}
}
