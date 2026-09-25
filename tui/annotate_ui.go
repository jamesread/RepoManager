package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/store"
)

// annotateUI edits one annotation on one project:
//
//	pick key  ->  (optionally name a new key)  ->  enter value
//
// If an Advanced Filter rule on an annotation field is active when it opens,
// the key stage is skipped and the prompt goes straight to that key. This is
// the "work through every project missing @status" workflow.
type annotateUI struct {
	open     bool
	stage    annotateStage
	projects []projectItem
	project  projectItem
	keyList  list.Model
	keyIn    textinput.Model
	valueIn  textinput.Model
	key      string
	width    int
	height   int

	// Existing values for the current key, ranked by usage. sugIdx is the
	// highlighted suggestion within the filtered view, or -1 for none.
	suggestions []valueCount
	sugIdx      int
}

// maxValueSuggestions caps how many existing values are listed under the prompt.
const maxValueSuggestions = 8

type annotateStage int

const (
	annotateStageKey annotateStage = iota
	annotateStageNewKey
	annotateStageValue
)

const annotateChoiceNewKey = "__new_key__"

type annotationSetMsg struct {
	paths []string
	key   string
	value string
	err   error
}

func newAnnotateUI() annotateUI {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Annotate"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down"))

	keyIn := textinput.New()
	keyIn.Placeholder = "new annotation key (e.g. status)"
	keyIn.CharLimit = 48
	keyIn.Width = 32

	valueIn := textinput.New()
	valueIn.Placeholder = "value (empty clears)"
	valueIn.CharLimit = 128
	valueIn.Width = 48

	return annotateUI{keyList: l, keyIn: keyIn, valueIn: valueIn}
}

func (u *annotateUI) setSize(w, h int) {
	u.width = w
	u.height = h
	u.keyList.SetSize(w, h-2)
}

// activeAnnotationKey returns the key of the first active annotation filter.
func (m *model) activeAnnotationKey() (string, bool) {
	for _, r := range m.advFilter.rules {
		if key, ok := annotationKeyFromField(r.FieldID); ok {
			return key, true
		}
	}
	return "", false
}

func (m *model) openAnnotate(items []projectItem) tea.Cmd {
	if len(items) == 0 {
		return nil
	}
	u := &m.annotate
	u.open = true
	u.projects = append([]projectItem{}, items...)
	u.project = items[0]
	if u.width > 0 {
		u.keyList.SetSize(u.width, u.height-2)
	}
	if key, ok := m.activeAnnotationKey(); ok {
		return m.annotateEnterValueStage(key)
	}
	m.populateAnnotateKeys()
	u.stage = annotateStageKey
	return nil
}

func (m *model) populateAnnotateKeys() {
	u := &m.annotate
	keys := m.store.AnnotationKeys()
	items := make([]list.Item, 0, len(keys)+1)
	for _, k := range keys {
		desc := "unset"
		if v, ok := u.project.annotations[k]; ok && v != "" {
			desc = v
		}
		items = append(items, filterChoiceItem{title: "@" + k, desc: desc, id: k})
	}
	items = append(items, filterChoiceItem{
		title: "+ new key…",
		desc:  "define a new global annotation key",
		id:    annotateChoiceNewKey,
	})
	u.keyList.Title = "Annotate · " + u.project.rel
	u.keyList.SetItems(items)
}

func (m *model) annotateEnterValueStage(key string) tea.Cmd {
	u := &m.annotate
	u.key = key
	u.stage = annotateStageValue
	u.suggestions = rankValueCounts(m.store.AnnotationValueCounts(key))
	u.sugIdx = -1
	u.valueIn.SetValue(u.project.annotations[key])
	u.valueIn.CursorEnd()
	u.valueIn.Focus()
	return textinput.Blink
}

// visibleSuggestions returns the suggestions matching the typed text. While a
// suggestion is highlighted the full list is shown so navigation is stable.
func (u annotateUI) visibleSuggestions() []valueCount {
	if u.sugIdx >= 0 {
		return u.suggestions
	}
	return filterValueCounts(u.suggestions, u.valueIn.Value())
}

// moveSuggestion highlights the suggestion delta steps away and copies it into
// the input. It wraps around and starts from the top on first use.
func (u *annotateUI) moveSuggestion(delta int) {
	visible := u.visibleSuggestions()
	if len(visible) == 0 {
		return
	}
	if u.sugIdx < 0 {
		// Switch to the stable full list, anchored on the first visible match.
		u.sugIdx = indexOfValue(u.suggestions, visible[0].value)
		if delta < 0 {
			u.sugIdx = (u.sugIdx - 1 + len(u.suggestions)) % len(u.suggestions)
		}
	} else {
		u.sugIdx = (u.sugIdx + delta + len(u.suggestions)) % len(u.suggestions)
	}
	u.valueIn.SetValue(u.suggestions[u.sugIdx].value)
	u.valueIn.CursorEnd()
}

func indexOfValue(list []valueCount, value string) int {
	for i, vc := range list {
		if vc.value == value {
			return i
		}
	}
	return 0
}

func (m model) updateAnnotate(msg tea.Msg) (tea.Model, tea.Cmd) {
	u := &m.annotate
	switch msg := msg.(type) {
	case engineEventMsg:
		return m, m.handleEngineEvent(msg.Event)
	case tea.WindowSizeMsg:
		m.resizeList(msg.Width-2, msg.Height-4)
		u.setSize(msg.Width-2, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		switch u.stage {
		case annotateStageKey:
			return m.updateAnnotateKey(msg)
		case annotateStageNewKey:
			return m.updateAnnotateNewKey(msg)
		case annotateStageValue:
			return m.updateAnnotateValue(msg)
		}
	}

	var cmd tea.Cmd
	switch u.stage {
	case annotateStageNewKey:
		u.keyIn, cmd = u.keyIn.Update(msg)
	case annotateStageValue:
		u.valueIn, cmd = u.valueIn.Update(msg)
	}
	return m, cmd
}

func (m model) updateAnnotateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := &m.annotate
	if u.keyList.FilterState() == list.Filtering {
		var cmd tea.Cmd
		u.keyList, cmd = u.keyList.Update(msg)
		return m, cmd
	}
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
		u.open = false
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		item, ok := u.keyList.SelectedItem().(filterChoiceItem)
		if !ok {
			return m, nil
		}
		if item.id == annotateChoiceNewKey {
			u.stage = annotateStageNewKey
			u.keyIn.SetValue("")
			u.keyIn.Focus()
			return m, textinput.Blink
		}
		return m, m.annotateEnterValueStage(item.id)
	}
	var cmd tea.Cmd
	u.keyList, cmd = u.keyList.Update(msg)
	return m, cmd
}

func (m model) updateAnnotateNewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := &m.annotate
	switch msg.Type {
	case tea.KeyEsc:
		u.keyIn.Blur()
		u.stage = annotateStageKey
		m.populateAnnotateKeys()
		return m, nil
	case tea.KeyEnter:
		key := store.NormalizeAnnotationKey(u.keyIn.Value())
		if key == "" {
			return m, nil
		}
		u.keyIn.Blur()
		if err := m.engSend(engine.AddAnnotationKeyRequest{Key: key}); err != nil {
			u.open = false
			return m, m.list.NewStatusMessage(fmt.Sprintf("annotation error: %v", err))
		}
		return m, m.annotateEnterValueStage(key)
	}
	var cmd tea.Cmd
	u.keyIn, cmd = u.keyIn.Update(msg)
	return m, cmd
}

func (m model) updateAnnotateValue(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := &m.annotate
	switch msg.Type {
	case tea.KeyEsc:
		u.valueIn.Blur()
		if _, filtered := m.activeAnnotationKey(); filtered {
			// Opened directly into the value stage; esc closes outright.
			u.open = false
			return m, nil
		}
		u.stage = annotateStageKey
		m.populateAnnotateKeys()
		return m, nil
	case tea.KeyEnter:
		paths := make([]string, len(u.projects))
		for i, p := range u.projects {
			paths[i] = p.path
		}
		key, value := u.key, strings.TrimSpace(u.valueIn.Value())
		u.valueIn.Blur()
		u.open = false
		return m, func() tea.Msg {
			err := m.engSend(engine.SetAnnotationRequest{Paths: paths, Key: key, Value: value})
			return annotationSetMsg{paths: paths, key: key, value: value, err: err}
		}
	case tea.KeyDown, tea.KeyTab, tea.KeyCtrlN:
		u.moveSuggestion(+1)
		return m, nil
	case tea.KeyUp, tea.KeyShiftTab, tea.KeyCtrlP:
		u.moveSuggestion(-1)
		return m, nil
	}
	// Any other key edits the text; drop the highlight so the list re-filters.
	u.sugIdx = -1
	var cmd tea.Cmd
	u.valueIn, cmd = u.valueIn.Update(msg)
	return m, cmd
}

func (u annotateUI) renderSuggestions() string {
	visible := u.visibleSuggestions()
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	if len(visible) == 0 {
		if len(u.suggestions) == 0 {
			return dim.Render("no existing values for @" + u.key)
		}
		return dim.Render("no existing values match")
	}

	// Keep the highlighted row on screen when the list is longer than the cap.
	start := 0
	if u.sugIdx >= maxValueSuggestions {
		start = u.sugIdx - maxValueSuggestions + 1
	}
	end := start + maxValueSuggestions
	if end > len(visible) {
		end = len(visible)
	}

	var b strings.Builder
	b.WriteString(dim.Render("existing values (↑/↓ or tab to pick):"))
	b.WriteString("\n")
	for i := start; i < end; i++ {
		vc := visible[i]
		line := fmt.Sprintf("%s (%d)", vc.value, vc.count)
		if u.sugIdx >= 0 && i == u.sugIdx {
			b.WriteString(cur.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	if hidden := len(visible) - (end - start); hidden > 0 {
		b.WriteString(dim.Render(fmt.Sprintf("  … %d more", hidden)))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (u annotateUI) titleLabel() string {
	if len(u.projects) > 1 {
		return fmt.Sprintf("%d projects", len(u.projects))
	}
	return u.project.rel
}

func (u annotateUI) View() string {
	if !u.open {
		return ""
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	switch u.stage {
	case annotateStageKey:
		return "\n" + u.keyList.View() + "\n" +
			dim.Render("enter pick key · / search · esc close")
	case annotateStageNewKey:
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Annotate · " + u.titleLabel()))
		b.WriteString("\n\n")
		b.WriteString(dim.Render("new key: "))
		b.WriteString(u.keyIn.View())
		b.WriteString("\n\n")
		b.WriteString(dim.Render("enter continue · esc back"))
		return b.String()
	default:
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Annotate · " + u.titleLabel()))
		b.WriteString("\n\n")
		current := u.project.annotations[u.key]
		if current == "" {
			b.WriteString(dim.Render(fmt.Sprintf("@%s is currently unset", u.key)))
		} else {
			b.WriteString(dim.Render(fmt.Sprintf("@%s is currently %q", u.key, current)))
		}
		b.WriteString("\n\n")
		b.WriteString(annotationStyle.Render("@" + u.key + " = "))
		b.WriteString(u.valueIn.View())
		b.WriteString("\n\n")
		b.WriteString(u.renderSuggestions())
		b.WriteString("\n\n")
		b.WriteString(dim.Render("enter save (empty clears) · ↑/↓ pick existing · esc back"))
		return b.String()
	}
}
