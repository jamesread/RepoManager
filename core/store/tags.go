package store

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"

	"github.com/jamesread/RepoManager/core/jsonfile"
)

// Stable palette of distinct terminal colors for tags.
var tagPalette = []string{
	"10",  // bright green
	"12",  // bright blue
	"13",  // bright magenta
	"14",  // bright cyan
	"11",  // bright yellow
	"9",   // bright red
	"6",   // cyan
	"5",   // magenta
	"4",   // blue
	"3",   // yellow
	"2",   // green
	"1",   // red
	"35",  // purple
	"208", // orange
	"45",  // turquoise
	"219", // pink
}

// Built-in tag colors applied on first load (and kept stable thereafter).
// Git uses a darker green so contrasting text resolves to white.
var defaultTagColors = map[string]string{
	"Git":    "28", // dark green → white fg
	"GitHub": "55", // deep purple → white fg
}

// legacyTagColors are migrated to the current defaults when found.
var legacyTagColors = map[string]string{
	"Git": "10", // old bright green paired poorly (black on lime)
}

func seedDefaultTagColors(colors map[string]string) {
	for name, color := range defaultTagColors {
		if legacy, ok := legacyTagColors[name]; ok {
			if current, exists := colors[name]; !exists || current == legacy {
				colors[name] = color
				continue
			}
		}
		if _, ok := colors[name]; !ok {
			colors[name] = color
		}
	}
}

// NormalizeTag trims surrounding whitespace from a tag name.
func NormalizeTag(tag string) string {
	return strings.TrimSpace(tag)
}

func colorForTagName(tag string) string {
	if color, ok := defaultTagColors[tag]; ok {
		return color
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(tag)))
	return tagPalette[int(h.Sum32())%len(tagPalette)]
}

// EnsureTagColor returns the persistent color for a tag, assigning one if new.
func (s *Store) EnsureTagColor(tag string) string {
	s.mu.Lock()
	color, changed := s.ensureTagColorLocked(tag)
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()

	if changed {
		_ = jsonfile.Write(path, settings)
	}
	return color
}

// ColorForTag is an alias of EnsureTagColor kept for readability at call sites.
func (s *Store) ColorForTag(tag string) string {
	return s.EnsureTagColor(tag)
}

// EnsureTagPastels returns a pastel #rrggbb for each tag, assigning and
// saving a color the first time a name is seen.
func (s *Store) EnsureTagPastels(tags []string) map[string]string {
	s.mu.Lock()
	if s.Settings.TagPastels == nil {
		s.Settings.TagPastels = map[string]string{}
	}
	changed := false
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		tag = NormalizeTag(tag)
		if tag == "" {
			continue
		}
		if color, ok := s.Settings.TagPastels[tag]; ok && color != "" {
			out[tag] = color
			continue
		}
		color := pastelForTagName(tag)
		s.Settings.TagPastels[tag] = color
		out[tag] = color
		changed = true
	}
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()

	if changed {
		_ = jsonfile.Write(path, settings)
	}
	return out
}

// pastelForTagName hashes the tag name into a light, muted color.
func pastelForTagName(tag string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(tag)))
	n := h.Sum32()
	hue := int(n % 360)
	sat := 46 + int((n/360)%16)   // 46–61%
	light := 80 + int((n/5760)%7) // 80–86%
	return hslToHex(hue, sat, light)
}

func hslToHex(hue, sat, light int) string {
	s := float64(sat) / 100
	l := float64(light) / 100
	c := (1 - math.Abs(2*l-1)) * s
	hp := float64(hue) / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return fmt.Sprintf("#%02x%02x%02x", channelByte(r+m), channelByte(g+m), channelByte(b+m))
}

func channelByte(v float64) int {
	n := int(math.Round(v * 255))
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

// SortTagsByOrder returns tags ordered by their index in order (lower = more
// important). Unranked tags follow, sorted case-insensitively.
func SortTagsByOrder(tags, order []string) []string {
	rank := make(map[string]int, len(order))
	for i, tag := range order {
		if _, dup := rank[tag]; !dup {
			rank[tag] = i
		}
	}
	out := append([]string{}, tags...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, iOK := rank[out[i]]
		rj, jOK := rank[out[j]]
		switch {
		case iOK && jOK:
			return ri < rj
		case iOK:
			return true
		case jOK:
			return false
		default:
			return strings.ToLower(out[i]) < strings.ToLower(out[j])
		}
	})
	return out
}
