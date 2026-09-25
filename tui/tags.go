package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
	"github.com/muesli/termenv"
)

// maxVisibleTags caps how many tag chips are drawn per project row.
const maxVisibleTags = 3

// splitVisibleTags returns the tags to draw and how many were hidden.
func splitVisibleTags(sorted []string, max int) (visible []string, hidden int) {
	if max <= 0 || len(sorted) <= max {
		return sorted, 0
	}
	return sorted[:max], len(sorted) - max
}

// moveTag shifts the tag at index i by delta positions, clamped to bounds.
func moveTag(order []string, i, delta int) []string {
	j := i + delta
	if i < 0 || i >= len(order) || j < 0 || j >= len(order) || i == j {
		return order
	}
	out := append([]string{}, order...)
	out[i], out[j] = out[j], out[i]
	return out
}

// contrastingForeground picks black or white text for WCAG-style contrast on bg.
func contrastingForeground(bgANSI string) string {
	n, err := strconv.Atoi(bgANSI)
	if err != nil || n < 0 || n > 255 {
		return "15"
	}
	rgb := termenv.ConvertToRGB(termenv.ANSI256Color(n))
	lum := relativeLuminance(rgb.R, rgb.G, rgb.B)
	// Contrast ratios against black (0) and white (1).
	contrastBlack := (lum + 0.05) / 0.05
	contrastWhite := 1.05 / (lum + 0.05)
	if contrastBlack >= contrastWhite {
		return "0"
	}
	return "15"
}

// renderProjectTags draws the most important tags as colored chips followed
// by a "+N" overflow marker when more are hidden.
func renderProjectTags(st *store.Store, tags []string) string {
	if len(tags) == 0 || st == nil {
		return ""
	}
	visible, hidden := splitVisibleTags(st.SortTagsByImportance(tags), maxVisibleTags)
	parts := make([]string, 0, len(visible)+1)
	for _, tag := range visible {
		parts = append(parts, renderTagChip(st, tag))
	}
	if hidden > 0 {
		parts = append(parts, tagOverflowStyle.Render("+"+strconv.Itoa(hidden)))
	}
	return strings.Join(parts, " ")
}

func renderTagChip(st *store.Store, tag string) string {
	if st == nil || tag == "" {
		return tag
	}
	bg := st.ColorForTag(tag)
	fg := contrastingForeground(bg)
	return tagChipStyle.
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Render(tag)
}

func relativeLuminance(r, g, b float64) float64 {
	return 0.2126*linearizeChannel(r) + 0.7152*linearizeChannel(g) + 0.0722*linearizeChannel(b)
}

func linearizeChannel(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func (m *model) syncGitStatusTags(path string, want []string) {
	wantSet := map[string]bool{}
	for _, tag := range want {
		tag = store.NormalizeTag(tag)
		if tag != "" {
			wantSet[tag] = true
		}
	}
	for _, name := range helpers.GitStatusTagNames {
		if wantSet[name] {
			_, _ = m.store.AddTag(path, name)
		} else {
			_, _ = m.store.RemoveTag(path, name)
		}
	}
}
