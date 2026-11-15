package ircfmt

import "strings"

const (
	bold  = "\x02"
	italic = "\x1d"
	strike = "\x1e"
	reset = "\x0f"
)

// ToIRC maps the marks IRC carries onto mIRC bytes. Other marks are dropped to their text.
func ToIRC(markdown string) string {
	s := markdown
	s = replacePair(s, "**", bold)
	s = replacePair(s, "~~", strike)
	s = replacePair(s, "_", italic)
	s = strip(s, "`")
	s = strip(s, "# ")
	s = strip(s, "> ")
	s = strip(s, "- ")
	return s
}

// FromIRC maps mIRC bytes back to markdown.
func FromIRC(s string) string {
	s = strings.ReplaceAll(s, bold, "**")
	s = strings.ReplaceAll(s, italic, "_")
	s = strings.ReplaceAll(s, strike, "~~")
	s = strings.ReplaceAll(s, reset, "")
	return s
}

func replacePair(s, mark, code string) string {
	parts := strings.Split(s, mark)
	if len(parts) < 3 {
		return s
	}
	var b strings.Builder
	for i, part := range parts {
		if i%2 == 1 && i != len(parts)-1 {
			b.WriteString(code)
			b.WriteString(part)
			b.WriteString(code)
			continue
		}
		b.WriteString(part)
	}
	return b.String()
}

func strip(s, mark string) string {
	return strings.ReplaceAll(s, mark, "")
}
