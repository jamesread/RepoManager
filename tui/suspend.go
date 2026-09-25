package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

func suspendCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.Suspend()
	}
}

// handleSuspend catches ctrl+z and resume after shell backgrounding. When
// handled, the second return value is true and the caller should return early.
func (m model) handleSuspend(msg tea.Msg) (model, tea.Cmd, bool) {
	switch msg.(type) {
	case tea.ResumeMsg:
		next, cmd := m.afterResume()
		return next, cmd, true
	}
	if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyCtrlZ {
		return m, suspendCmd(), true
	}
	return m, nil, false
}

func (m model) afterResume() (model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.scanning {
		cmds = append(cmds, m.list.StartSpinner())
	}
	cmds = append(cmds, waitEvent(m.events))
	return m, tea.Batch(cmds...)
}
