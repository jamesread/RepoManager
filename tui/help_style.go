package main

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const helpEnterSymbol = "↵"

func isEnterKey(keyText string) bool {
	return strings.EqualFold(strings.TrimSpace(keyText), "enter")
}

func primaryHelpKeyChar(keyText string) string {
	for _, r := range keyText {
		if r != ' ' {
			return string(r)
		}
	}
	if keyText == "" {
		return ""
	}
	_, size := utf8.DecodeRuneInString(keyText)
	return keyText[:size]
}

func helpKeyCharsToTry(keyText string) []string {
	if isEnterKey(keyText) {
		return nil
	}
	var chars []string
	seen := map[rune]struct{}{}
	for _, r := range keyText {
		if r == ' ' || r == '/' || r == '↑' || r == '↓' {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		chars = append(chars, string(r))
	}
	return chars
}

func isWordStart(s string, i int) bool {
	if i > 0 {
		prev, _ := utf8.DecodeLastRuneInString(s[:i])
		if !unicode.IsSpace(prev) {
			return false
		}
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func embedHelpKeyInDesc(keyText, desc string) (prefix, highlight, suffix string, ok bool) {
	for _, ch := range helpKeyCharsToTry(keyText) {
		kr, _ := utf8.DecodeRuneInString(ch)
		i := 0
		for i < len(desc) {
			for i < len(desc) && !isWordStart(desc, i) {
				i++
			}
			if i >= len(desc) {
				break
			}
			wr, wsize := utf8.DecodeRuneInString(desc[i:])
			if wr == kr || strings.EqualFold(string(wr), string(kr)) {
				return desc[:i], ch, desc[i+wsize:], true
			}
			for i < len(desc) && !unicode.IsSpace(rune(desc[i])) {
				_, size := utf8.DecodeRuneInString(desc[i:])
				i += size
			}
		}
	}
	return "", "", "", false
}

func styledHelpFallback(keyText, desc string, keyStyle, descStyle lipgloss.Style) string {
	primary := primaryHelpKeyChar(keyText)
	if desc == "" {
		return keyStyle.Render(keyText)
	}
	if len(keyText) > len(primary) {
		tail := keyText[len(primary):]
		if tail != "" && !strings.HasSuffix(tail, " ") {
			tail += " "
		}
		return keyStyle.Render(primary) + descStyle.Render(tail+desc)
	}
	return keyStyle.Render(primary) + descStyle.Render(" "+desc)
}

func styledShortHelpItem(kb key.Binding, styles help.Styles) string {
	keyStyle := styles.ShortKey.Inline(true)
	descStyle := styles.ShortDesc.Inline(true)
	keyText := kb.Help().Key
	desc := kb.Help().Desc

	if isEnterKey(keyText) {
		return keyStyle.Render(helpEnterSymbol) + descStyle.Render(" "+desc)
	}

	if prefix, highlight, suffix, ok := embedHelpKeyInDesc(keyText, desc); ok {
		return descStyle.Render(prefix) + keyStyle.Render(highlight) + descStyle.Render(suffix)
	}

	return styledHelpFallback(keyText, desc, keyStyle, descStyle)
}

func styledShortHelpView(h help.Model, bindings []key.Binding) string {
	if len(bindings) == 0 {
		return ""
	}

	var b strings.Builder
	var totalWidth int
	separator := h.Styles.ShortSeparator.Inline(true).Render(h.ShortSeparator)

	for i, kb := range bindings {
		if !kb.Enabled() {
			continue
		}

		sep := ""
		if totalWidth > 0 && i < len(bindings) {
			sep = separator
		}

		str := sep + styledShortHelpItem(kb, h.Styles)
		w := lipgloss.Width(str)

		if h.Width > 0 && totalWidth+w > h.Width {
			tail := " " + h.Styles.Ellipsis.Inline(true).Render(h.Ellipsis)
			if totalWidth+lipgloss.Width(tail) < h.Width {
				b.WriteString(tail)
			}
			break
		}

		totalWidth += w
		b.WriteString(str)
	}

	return b.String()
}
