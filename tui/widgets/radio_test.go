package widgets

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func threeOpts() RadioGroup {
	return NewRadioGroup("Size",
		RadioOption{ID: "s", Label: "Small", Desc: "small one"},
		RadioOption{ID: "m", Label: "Medium", Desc: "medium one"},
		RadioOption{ID: "l", Label: "Large", Desc: "large one"},
	)
}

func TestRadioGroupInlineDecision(t *testing.T) {
	r := threeOpts()
	need := r.inlineWidth()
	if need == 0 {
		t.Fatal("inline width should be positive")
	}
	if !r.Inline(0) {
		t.Fatal("unknown width should allow inline")
	}
	if !r.Inline(need) {
		t.Fatal("exact width should allow inline")
	}
	if r.Inline(need - 1) {
		t.Fatal("one column short should stack")
	}

	four := threeOpts()
	four.Options = append(four.Options, RadioOption{ID: "x", Label: "XL"})
	if four.Inline(1000) {
		t.Fatal("more than three options must stack regardless of width")
	}
}

func TestRadioGroupViewShapes(t *testing.T) {
	r := threeOpts()
	r.Select("m")

	inline := r.View(200)
	lines := strings.Split(strings.TrimRight(inline, "\n"), "\n")
	if len(lines) != 2 { // options row + selected description
		t.Fatalf("inline view should be 2 lines, got %d:\n%s", len(lines), inline)
	}
	if !strings.Contains(lines[0], RadioGlyphOn+" Medium") || !strings.Contains(lines[0], RadioGlyphOff+" Small") {
		t.Fatalf("inline row wrong: %q", lines[0])
	}
	// Options start after the 2-column marker, the padded title and the gap.
	want := 2 + RadioTitleWidth + len(RadioTitleGap)
	if idx := strings.Index(lines[0], RadioGlyphOff); idx != want {
		t.Fatalf("options should start at column %d, got %d: %q", want, idx, lines[0])
	}

	long := NewRadioGroup(strings.Repeat("x", RadioTitleWidth+5), RadioOption{ID: "a", Label: "A"})
	if !strings.Contains(long.View(200), strings.Repeat("x", RadioTitleWidth+5)+RadioTitleGap+RadioGlyphOn) {
		t.Fatal("over-long title must not be truncated and keeps its gap")
	}
	if !strings.Contains(lines[1], "medium one") {
		t.Fatalf("should describe the selected option: %q", lines[1])
	}

	stacked := r.View(10)
	lines = strings.Split(strings.TrimRight(stacked, "\n"), "\n")
	if len(lines) != 5 { // title + 3 options + description
		t.Fatalf("stacked view should be 5 lines, got %d:\n%s", len(lines), stacked)
	}
	if !strings.HasSuffix(strings.TrimSpace(lines[0]), "Size") {
		t.Fatalf("stacked title line wrong: %q", lines[0])
	}
	if !strings.Contains(lines[2], RadioGlyphOn+" Medium") {
		t.Fatalf("stacked selected option wrong: %q", lines[2])
	}

	r.Focused = true
	if !strings.Contains(r.View(200), "> ") {
		t.Fatal("focused group should show the cursor marker")
	}
}

func TestRadioGroupUpdateCycles(t *testing.T) {
	r := threeOpts()
	right := tea.KeyMsg{Type: tea.KeyRight}
	left := tea.KeyMsg{Type: tea.KeyLeft}
	space := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}

	if c, ch := r.Update(right); !c || !ch || r.SelectedID() != "m" {
		t.Fatalf("right: consumed=%v changed=%v id=%s", c, ch, r.SelectedID())
	}
	if _, _ = r.Update(space); r.SelectedID() != "l" {
		t.Fatalf("space should advance, got %s", r.SelectedID())
	}
	if _, _ = r.Update(right); r.SelectedID() != "s" {
		t.Fatalf("should wrap to first, got %s", r.SelectedID())
	}
	if _, _ = r.Update(left); r.SelectedID() != "l" {
		t.Fatalf("left should wrap to last, got %s", r.SelectedID())
	}
	if c, _ := r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}); c {
		t.Fatal("j is not a radio key and must not be consumed")
	}
	if !r.Select("m") || r.SelectedID() != "m" {
		t.Fatal("Select by id")
	}
	if r.Select("nope") {
		t.Fatal("unknown id must not select")
	}
}
