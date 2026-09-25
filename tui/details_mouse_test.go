package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDetailsRightClickBack(t *testing.T) {
	m := testMouseModel(t, nil, layoutComfy)
	m.details.open = true
	m.details.path = "/tmp/proj"

	cmd, handled := m.handleDetailsMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})
	if !handled {
		t.Fatal("expected right click to be handled")
	}
	if cmd != nil {
		t.Fatal("back should not return a command")
	}
	if m.details.open {
		t.Fatal("expected details to close on right click")
	}
}
