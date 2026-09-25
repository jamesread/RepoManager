package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jamesread/RepoManager/core/engine"
)

func TestSuspendCmd(t *testing.T) {
	msg := suspendCmd()()
	if _, ok := msg.(tea.SuspendMsg); !ok {
		t.Fatalf("got %T", msg)
	}
}

func TestHandleSuspendCtrlZ(t *testing.T) {
	m := model{}
	_, cmd, ok := m.handleSuspend(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if !ok || cmd == nil {
		t.Fatal("expected ctrl+z to suspend")
	}
	if _, ok := cmd().(tea.SuspendMsg); !ok {
		t.Fatalf("cmd returned %T", cmd())
	}
}

func TestHandleSuspendResume(t *testing.T) {
	events := make(chan engine.Event, 1)
	m := model{events: events, scanning: true}
	_, cmd, ok := m.handleSuspend(tea.ResumeMsg{})
	if !ok || cmd == nil {
		t.Fatal("expected resume handling")
	}
}
