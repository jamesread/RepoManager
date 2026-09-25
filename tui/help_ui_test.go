package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestHelpContextList(t *testing.T) {
	m := model{list: list.New(nil, list.NewDefaultDelegate(), 80, 24)}
	if m.currentHelpContext() != helpContextList {
		t.Fatalf("context=%v", m.currentHelpContext())
	}
	m.openHelp()
	if m.help.title != "Help · project list" {
		t.Fatalf("title=%q", m.help.title)
	}
	if !strings.Contains(strings.Join(m.help.lines, "\n"), "advanced filter") {
		t.Fatal("expected list help content")
	}
}

func TestHelpContextDetails(t *testing.T) {
	m := model{
		list:    list.New(nil, list.NewDefaultDelegate(), 80, 24),
		details: detailsUI{open: true},
	}
	m.openHelp()
	if m.help.title != "Help · project details" {
		t.Fatalf("title=%q", m.help.title)
	}
	if !strings.Contains(strings.Join(m.help.lines, "\n"), "yank project details") {
		t.Fatal("expected details help content")
	}
}

func TestHelpContextAdvFilterMulti(t *testing.T) {
	m := model{
		list: list.New(nil, list.NewDefaultDelegate(), 80, 24),
		advFilter: advancedFilter{
			open:        true,
			stage:       filterStageValues,
			multiSelect: true,
		},
	}
	m.openHelp()
	if m.help.title != "Help · filter values (multi)" {
		t.Fatalf("title=%q", m.help.title)
	}
	if !strings.Contains(strings.Join(m.help.lines, "\n"), "[+] match") {
		t.Fatal("expected multi filter help content")
	}
}
