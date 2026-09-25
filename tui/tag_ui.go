package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
)

const maxTagSuggestions = 8

func (m *model) openTagging(items []projectItem) tea.Cmd {
	if len(items) == 0 {
		return nil
	}
	m.tagging = true
	m.tagTargets = append([]projectItem{}, items...)
	m.tagInput.SetValue("")
	m.tagSugIdx = -1
	m.populateTagSuggestions(items)
	m.tagInput.Focus()
	return textinput.Blink
}

func (m *model) populateTagSuggestions(items []projectItem) {
	if len(items) == 0 {
		m.tagSuggestions = nil
		return
	}
	hasTag := map[string]int{}
	for _, item := range items {
		seen := map[string]struct{}{}
		for _, tag := range item.tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			hasTag[tag]++
		}
	}
	counts := m.store.TagCounts()
	out := make([]valueCount, 0, len(counts))
	for _, tag := range m.store.AllKnownTags() {
		if helpers.IsGitStatusTag(tag) || hasTag[tag] == len(items) {
			continue
		}
		out = append(out, valueCount{value: tag, count: counts[tag]})
	}
	m.tagSuggestions = out
}

func (m *model) visibleTagSuggestions() []valueCount {
	if m.tagSugIdx >= 0 {
		return m.tagSuggestions
	}
	return filterValueCounts(m.tagSuggestions, m.tagInput.Value())
}

func (m *model) moveTagSuggestion(delta int) {
	visible := m.visibleTagSuggestions()
	if len(visible) == 0 {
		return
	}
	if m.tagSugIdx < 0 {
		m.tagSugIdx = indexOfValue(m.tagSuggestions, visible[0].value)
		if delta < 0 {
			m.tagSugIdx = (m.tagSugIdx - 1 + len(m.tagSuggestions)) % len(m.tagSuggestions)
		}
	} else {
		m.tagSugIdx = (m.tagSugIdx + delta + len(m.tagSuggestions)) % len(m.tagSuggestions)
	}
	m.tagInput.SetValue(m.tagSuggestions[m.tagSugIdx].value)
	m.tagInput.CursorEnd()
}

func (m model) renderTagSuggestions() string {
	visible := m.visibleTagSuggestions()
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	if len(visible) == 0 {
		if len(m.tagSuggestions) == 0 {
			return dim.Render("no existing tags")
		}
		return dim.Render("no existing tags match")
	}

	start := 0
	if m.tagSugIdx >= maxTagSuggestions {
		start = m.tagSugIdx - maxTagSuggestions + 1
	}
	end := start + maxTagSuggestions
	if end > len(visible) {
		end = len(visible)
	}

	var b strings.Builder
	b.WriteString(dim.Render("existing tags (↑/↓ or tab to pick):"))
	b.WriteString("\n")
	for i := start; i < end; i++ {
		vc := visible[i]
		line := renderTagChip(m.store, vc.value)
		if vc.count > 0 {
			line += dim.Render(fmt.Sprintf(" (%d)", vc.count))
		}
		selected := m.tagSugIdx >= 0 && i == m.tagSugIdx
		if selected {
			b.WriteString(cur.Render("> "))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) renderTagPrompt() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	var b strings.Builder
	b.WriteString("\n")
	if len(m.tagTargets) > 1 {
		b.WriteString(dim.Render(fmt.Sprintf("tag %d projects: ", len(m.tagTargets))))
	} else {
		b.WriteString(dim.Render("tag: "))
	}
	b.WriteString(m.tagInput.View())
	if sug := m.renderTagSuggestions(); sug != "" {
		b.WriteString("\n")
		b.WriteString(sug)
	}
	return b.String()
}

func (m model) updateTagging(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.tagging = false
			m.tagInput.Blur()
			m.tagSugIdx = -1
			m.tagTargets = nil
			return m, nil
		case tea.KeyEnter:
			tag := store.NormalizeTag(m.tagInput.Value())
			targets := append([]projectItem{}, m.tagTargets...)
			m.tagging = false
			m.tagInput.Blur()
			m.tagSugIdx = -1
			m.tagTargets = nil
			if tag == "" || len(targets) == 0 {
				return m, nil
			}
			paths := make([]string, len(targets))
			for i, item := range targets {
				paths[i] = item.path
			}
			return m, func() tea.Msg {
				err := m.engSend(engine.AddTagRequest{Paths: paths, Tag: tag})
				return tagAddedMsg{paths: paths, tag: tag, err: err}
			}
		case tea.KeyDown, tea.KeyTab, tea.KeyCtrlN:
			m.moveTagSuggestion(+1)
			return m, nil
		case tea.KeyUp, tea.KeyShiftTab, tea.KeyCtrlP:
			m.moveTagSuggestion(-1)
			return m, nil
		}
		m.tagSugIdx = -1
	}

	var cmd tea.Cmd
	m.tagInput, cmd = m.tagInput.Update(msg)
	return m, cmd
}
