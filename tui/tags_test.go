package main

import (
	"strings"
	"testing"
)

func TestContrastingForeground(t *testing.T) {
	// Bright yellow / lime backgrounds need dark text.
	if got := contrastingForeground("11"); got != "0" {
		t.Fatalf("bright yellow fg=%q, want 0", got)
	}
	if got := contrastingForeground("10"); got != "0" {
		t.Fatalf("bright green fg=%q, want 0", got)
	}
	// Darker greens (incl. Git) and dark colors need light text.
	if got := contrastingForeground("28"); got != "15" {
		t.Fatalf("dark green fg=%q, want 15", got)
	}
	if got := contrastingForeground("4"); got != "15" {
		t.Fatalf("blue fg=%q, want 15", got)
	}
	if got := contrastingForeground("1"); got != "15" {
		t.Fatalf("red fg=%q, want 15", got)
	}
}

func TestSplitVisibleTags(t *testing.T) {
	vis, hidden := splitVisibleTags([]string{"a", "b"}, 3)
	if len(vis) != 2 || hidden != 0 {
		t.Fatalf("under cap: vis=%v hidden=%d", vis, hidden)
	}
	vis, hidden = splitVisibleTags([]string{"a", "b", "c", "d", "e"}, 3)
	if strings.Join(vis, ",") != "a,b,c" || hidden != 2 {
		t.Fatalf("over cap: vis=%v hidden=%d", vis, hidden)
	}
}

func TestMoveTag(t *testing.T) {
	order := []string{"a", "b", "c"}
	got := moveTag(order, 2, -1)
	if strings.Join(got, ",") != "a,c,b" {
		t.Fatalf("move up: %v", got)
	}
	if strings.Join(order, ",") != "a,b,c" {
		t.Fatalf("input mutated: %v", order)
	}
	if got := moveTag(order, 0, -1); strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("clamp at top: %v", got)
	}
	if got := moveTag(order, 2, 1); strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("clamp at bottom: %v", got)
	}
}
