// Package widgets holds reusable Bubble Tea controls that carry no
// application state. Each control follows the Update/View shape: the parent
// forwards key messages, reads the result, and splices View into its output.
package widgets

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RadioOption is one choice in a RadioGroup.
type RadioOption struct {
	ID    string
	Label string
	Desc  string
}

// RadioGroup is a single-choice control that shows every option at once,
// like a select box that is always expanded. With three or fewer options and
// enough width the options sit on one line; otherwise they stack vertically.
//
// It follows the Bubble Tea shape: the parent forwards key messages to
// Update, reads Selected/SelectedID, and splices View into its own output.
type RadioGroup struct {
	Title    string
	Options  []RadioOption
	Selected int
	// Focused draws the group as the active control (cursor marker, bold).
	Focused bool
}

// RadioMaxInline is the largest option count drawn on a single line.
const RadioMaxInline = 3

// RadioTitleWidth pads the setting name so option columns line up across
// several radio groups in one menu.
const RadioTitleWidth = 25

const (
	RadioGlyphOn  = "(•)"
	RadioGlyphOff = "( )"
	// radioInlineGap separates options on the inline row.
	radioInlineGap = "   "
)

var (
	radioNext = key.NewBinding(key.WithKeys("right", "tab", " ", "enter"))
	radioPrev = key.NewBinding(key.WithKeys("left", "shift+tab"))
)

func NewRadioGroup(title string, opts ...RadioOption) RadioGroup {
	return RadioGroup{Title: title, Options: opts}
}

// SelectedID returns the ID of the current choice, or "" when empty.
func (r RadioGroup) SelectedID() string {
	if r.Selected < 0 || r.Selected >= len(r.Options) {
		return ""
	}
	return r.Options[r.Selected].ID
}

// Select makes the option with the given ID current. It reports whether the
// ID was found.
func (r *RadioGroup) Select(id string) bool {
	for i, o := range r.Options {
		if o.ID == id {
			r.Selected = i
			return true
		}
	}
	return false
}

// Next and Prev cycle the selection, wrapping at either end.
func (r *RadioGroup) Next() {
	if n := len(r.Options); n > 0 {
		r.Selected = (r.Selected + 1) % n
	}
}

func (r *RadioGroup) Prev() {
	if n := len(r.Options); n > 0 {
		r.Selected = (r.Selected - 1 + n) % n
	}
}

// Update handles a key press. It reports whether the key was consumed and
// whether the selection changed, so the parent can react (persist, redraw)
// without diffing state.
func (r *RadioGroup) Update(msg tea.KeyMsg) (consumed, changed bool) {
	before := r.Selected
	switch {
	case key.Matches(msg, radioNext):
		r.Next()
	case key.Matches(msg, radioPrev):
		r.Prev()
	default:
		return false, false
	}
	return true, r.Selected != before
}

// RadioTitleGap always separates the padded title from the first option.
const RadioTitleGap = "  "

// paddedTitle returns the title padded (never truncated) to RadioTitleWidth,
// followed by RadioTitleGap.
func (r RadioGroup) paddedTitle() string {
	title := r.Title
	if pad := RadioTitleWidth - lipgloss.Width(title); pad > 0 {
		title += strings.Repeat(" ", pad)
	}
	return title + RadioTitleGap
}

// inlineWidth is the width of the single-row form: cursor marker, padded
// title, and every option.
func (r RadioGroup) inlineWidth() int {
	w := 2 + lipgloss.Width(r.paddedTitle())
	for i, o := range r.Options {
		if i > 0 {
			w += len(radioInlineGap)
		}
		w += lipgloss.Width(RadioGlyphOn) + 1 + lipgloss.Width(o.Label)
	}
	return w
}

// Inline reports whether the options fit on one row at the given width.
// A non-positive width means "unknown" and is treated as unconstrained.
func (r RadioGroup) Inline(width int) bool {
	if len(r.Options) > RadioMaxInline {
		return false
	}
	if width <= 0 {
		return true
	}
	return r.inlineWidth() <= width
}

// View renders the control. width is the space available for the whole
// row, including the cursor marker and title; it drives the inline decision.
func (r RadioGroup) View(width int) string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cur := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	marker := "  "
	if r.Focused {
		marker = cur.Render("> ")
	}

	option := func(i int) string {
		o := r.Options[i]
		glyph := RadioGlyphOff
		if i == r.Selected {
			glyph = RadioGlyphOn
		}
		text := glyph + " " + o.Label
		if i == r.Selected && r.Focused {
			return cur.Render(text)
		}
		return text
	}

	var b strings.Builder
	if r.Inline(width) {
		parts := make([]string, 0, len(r.Options))
		for i := range r.Options {
			parts = append(parts, option(i))
		}
		b.WriteString(marker + r.paddedTitle() + strings.Join(parts, radioInlineGap) + "\n")
	} else {
		b.WriteString(marker + r.Title + "\n")
		for i := range r.Options {
			b.WriteString("    " + option(i) + "\n")
		}
	}
	if desc := r.selectedDesc(); desc != "" {
		b.WriteString("    " + dim.Render(desc) + "\n")
	}
	return b.String()
}

func (r RadioGroup) selectedDesc() string {
	if r.Selected < 0 || r.Selected >= len(r.Options) {
		return ""
	}
	return r.Options[r.Selected].Desc
}
