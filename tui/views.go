package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

const (
	viewIDPrefixPreset = "preset:"
	viewIDPrefixSaved  = "saved:"
)

type predefinedView struct {
	ID     string
	Name   string
	Desc   string
	Config store.ViewConfig
}

var predefinedViews = []predefinedView{
	{
		ID:   viewIDPrefixPreset + "all",
		Name: "All projects",
		Desc: "default layout, no filters",
		Config: store.ViewConfig{
			Layout:       string(layoutComfy),
			PathRelative: true,
			PathTilde:    true,
			SortFieldID:  "name",
		},
	},
	{
		ID:   viewIDPrefixPreset + "git",
		Name: "Git repos",
		Desc: "filter · tag Git",
		Config: store.ViewConfig{
			Layout:       string(layoutComfy),
			PathRelative: true,
			PathTilde:    true,
			SortFieldID:  "name",
			AdvFilter: []store.FilterRule{
				{FieldID: "tags", Values: []string{"Git"}},
			},
		},
	},
	{
		ID:   viewIDPrefixPreset + "compact",
		Name: "Compact list",
		Desc: "one line per project",
		Config: store.ViewConfig{
			Layout:       string(layoutCompact),
			PathRelative: true,
			PathTilde:    true,
			SortFieldID:  "name",
		},
	},
	{
		ID:   viewIDPrefixPreset + "recent",
		Name: "Recently modified",
		Desc: "sort by directory mtime, newest first",
		Config: store.ViewConfig{
			Layout:       string(layoutComfy),
			PathRelative: true,
			PathTilde:    true,
			SortFieldID:  helpers.FieldDirMtime,
			SortSpecs:    []store.SortSpec{{ID: helpers.FieldDirMtime, Desc: true}},
		},
	},
}

func predefinedViewByID(id string) (predefinedView, bool) {
	for _, v := range predefinedViews {
		if v.ID == id {
			return v, true
		}
	}
	return predefinedView{}, false
}

func isReservedViewName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	lower := strings.ToLower(name)
	for _, p := range predefinedViews {
		if strings.EqualFold(p.Name, name) || strings.EqualFold(strings.TrimPrefix(p.ID, viewIDPrefixPreset), name) {
			return true
		}
		if strings.ToLower(p.Name) == lower {
			return true
		}
	}
	return false
}

func savedViewID(name string) string {
	return viewIDPrefixSaved + name
}

func (m *model) currentViewConfig() store.ViewConfig {
	quick := ""
	if m.list.FilterState() == list.FilterApplied {
		quick = strings.TrimSpace(m.list.FilterValue())
	}
	return store.ViewConfig{
		Layout:       string(m.layout),
		PathRelative: m.store.PathRelative(),
		PathTilde:    m.store.PathTilde(),
		SortFieldID:  m.currentSort().ID,
		SortFieldIDs: m.sortFieldIDs(),
		SortSpecs:    m.effectiveSortSpecs(),
		QuickFilter:  quick,
		AdvFilter:    persistFilterRules(m.advFilter.rules),
	}
}

func (m *model) applyViewConfig(cfg store.ViewConfig) {
	mode := parseLayoutMode(cfg.Layout)
	if mode != m.layout {
		m.applyLayout(mode)
		_ = m.engSend(engine.SetLayoutRequest{Layout: string(mode)})
	}
	if cfg.PathRelative != m.store.PathRelative() {
		_ = m.engSend(engine.SetPathRelativeRequest{Relative: cfg.PathRelative})
		m.applyLayout(m.layout)
	}
	if cfg.PathTilde != m.store.PathTilde() {
		_ = m.engSend(engine.SetPathTildeRequest{Tilde: cfg.PathTilde})
		m.applyLayout(m.layout)
	}

	m.setSortSpecs(sortSpecsFromPersisted(cfg.SortSpecs, cfg.SortFieldIDs, cfg.SortFieldID))

	m.advFilter.rules = restoreFilterRules(m, cfg.AdvFilter)
	if cfg.QuickFilter != "" {
		m.list.SetFilterText(cfg.QuickFilter)
	} else {
		m.list.ResetFilter()
	}
}

func (m *model) loadViewByID(id string) error {
	if p, ok := predefinedViewByID(id); ok {
		m.applyViewConfig(p.Config)
		m.activeViewID = p.ID
		m.publishList()
		m.refreshTitle()
		m.persistSession()
		return nil
	}
	if strings.HasPrefix(id, viewIDPrefixSaved) {
		name := strings.TrimPrefix(id, viewIDPrefixSaved)
		sv, ok := m.store.ViewNamed(m.root, name)
		if !ok {
			return fmt.Errorf("view %q not found", name)
		}
		m.applyViewConfig(sv.Config)
		m.activeViewID = id
		m.publishList()
		m.refreshTitle()
		m.persistSession()
		return nil
	}
	return fmt.Errorf("unknown view %q", id)
}

func (m *model) activeViewTitle() string {
	if m.activeViewID == "" {
		return ""
	}
	if p, ok := predefinedViewByID(m.activeViewID); ok {
		return p.Name
	}
	if strings.HasPrefix(m.activeViewID, viewIDPrefixSaved) {
		return strings.TrimPrefix(m.activeViewID, viewIDPrefixSaved)
	}
	return m.activeViewID
}

func (m *model) saveCurrentView(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("view name required")
	}
	if isReservedViewName(name) {
		return fmt.Errorf("name %q is reserved", name)
	}
	cfg := m.currentViewConfig()
	if err := m.engSend(engine.SaveViewRequest{Name: name, Config: cfg}); err != nil {
		return err
	}
	m.activeViewID = savedViewID(name)
	m.persistSession()
	return nil
}
