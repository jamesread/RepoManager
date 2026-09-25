package main

import (
	"fmt"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

type clipboardMsg struct {
	status string
	err    error
}

func (m *model) copyText(text, status string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(text)
		return clipboardMsg{status: status, err: err}
	}
}

func (m *model) copyProjectPath(path string) tea.Cmd {
	label := path
	if len(label) > 72 {
		label = "…" + label[len(label)-69:]
	}
	return m.copyText(path, "copied · "+label)
}

func (m *model) yankProjectDetails(p projectItem) tea.Cmd {
	return m.copyText(m.projectDetailsMarkdown(p), "yanked · markdown")
}

func (m *model) clipboardStatus(msg clipboardMsg) tea.Cmd {
	if msg.err != nil {
		return m.list.NewStatusMessage(fmt.Sprintf("clipboard: %v", msg.err))
	}
	return m.list.NewStatusMessage(msg.status)
}
