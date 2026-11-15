package ui

import (
	"strings"

	"lonchan/internal/caps"
)

// span is a formatting range over the plain text the user sees.
type span struct {
	start int
	end   int
	mark  caps.Mark
}

// splitMarks removes the markers the composer inserts and records the ranges.
func splitMarks(markdown string) (string, []span) {
	rs := []rune(markdown)
	var plain []rune
	var spans []span
	bold, italic, strike, code := -1, -1, -1, -1
	lineStart := true
	for i := 0; i < len(rs); {
		if lineStart {
			lineStart = false
			if rest := string(rs[i:]); strings.HasPrefix(rest, "# ") {
				spans = append(spans, span{start: len(plain), end: len(plain), mark: caps.Heading})
				i += 2
				continue
			}
			if rest := string(rs[i:]); strings.HasPrefix(rest, "> ") {
				spans = append(spans, span{start: len(plain), end: len(plain), mark: caps.Quote})
				i += 2
				continue
			}
			if rest := string(rs[i:]); strings.HasPrefix(rest, "- ") {
				spans = append(spans, span{start: len(plain), end: len(plain), mark: caps.List})
				i += 2
				continue
			}
		}
		if i+1 < len(rs) && rs[i] == '*' && rs[i+1] == '*' {
			if bold < 0 {
				bold = len(plain)
			} else {
				spans = append(spans, span{start: bold, end: len(plain), mark: caps.Bold})
				bold = -1
			}
			i += 2
			continue
		}
		if i+1 < len(rs) && rs[i] == '~' && rs[i+1] == '~' {
			if strike < 0 {
				strike = len(plain)
			} else {
				spans = append(spans, span{start: strike, end: len(plain), mark: caps.Strike})
				strike = -1
			}
			i += 2
			continue
		}
		if rs[i] == '_' {
			if italic < 0 {
				italic = len(plain)
			} else {
				spans = append(spans, span{start: italic, end: len(plain), mark: caps.Italic})
				italic = -1
			}
			i++
			continue
		}
		if rs[i] == '`' {
			if code < 0 {
				code = len(plain)
			} else {
				spans = append(spans, span{start: code, end: len(plain), mark: caps.Code})
				code = -1
			}
			i++
			continue
		}
		plain = append(plain, rs[i])
		if rs[i] == '\n' {
			lineStart = true
			closeLine(&spans, len(plain)-1)
		}
		i++
	}
	closeLine(&spans, len(plain))
	return string(plain), spans
}

func closeLine(spans *[]span, end int) {
	for i := range *spans {
		s := &(*spans)[i]
		if (s.mark == caps.Heading || s.mark == caps.Quote || s.mark == caps.List) && s.end == s.start {
			s.end = end
		}
	}
}

// wire puts the markers back around the plain text for the protocols that carry them.
func wire(plain string, spans []span) string {
	rs := []rune(plain)
	type bit struct {
		at   int
		text string
		ord  int
	}
	var bits []bit
	for _, s := range spans {
		if s.end < s.start {
			continue
		}
		if s.start < 0 {
			s.start = 0
		}
		if s.end > len(rs) {
			s.end = len(rs)
		}
		switch s.mark {
		case caps.Heading:
			bits = append(bits, bit{at: lineStartAt(rs, s.start), text: "# ", ord: 0})
		case caps.Quote:
			bits = append(bits, bit{at: lineStartAt(rs, s.start), text: "> ", ord: 0})
		case caps.List:
			bits = append(bits, bit{at: lineStartAt(rs, s.start), text: "- ", ord: 0})
		default:
			open, close := markPair(s.mark)
			bits = append(bits, bit{at: s.start, text: open, ord: 1})
			bits = append(bits, bit{at: s.end, text: close, ord: 0})
		}
	}
	// Later inserts at the same index happen first so earlier text stays put. Stable by walking from the end.
	for i := 0; i < len(bits); i++ {
		for j := i + 1; j < len(bits); j++ {
			if bits[j].at > bits[i].at || (bits[j].at == bits[i].at && bits[j].ord > bits[i].ord) {
				bits[i], bits[j] = bits[j], bits[i]
			}
		}
	}
	out := append([]rune{}, rs...)
	for _, b := range bits {
		at := b.at
		if at < 0 {
			at = 0
		}
		if at > len(out) {
			at = len(out)
		}
		ins := []rune(b.text)
		next := make([]rune, 0, len(out)+len(ins))
		next = append(next, out[:at]...)
		next = append(next, ins...)
		next = append(next, out[at:]...)
		out = next
	}
	return string(out)
}

func lineStartAt(rs []rune, i int) int {
	if i > len(rs) {
		i = len(rs)
	}
	for i > 0 && rs[i-1] != '\n' {
		i--
	}
	return i
}

func markPair(mark caps.Mark) (string, string) {
	switch mark {
	case caps.Bold:
		return "**", "**"
	case caps.Italic:
		return "_", "_"
	case caps.Strike:
		return "~~", "~~"
	case caps.Code:
		return "`", "`"
	case caps.CodeBlock:
		return "```", "```"
	default:
		return "", ""
	}
}

// shiftSpans keeps ranges pointed at the same words after plain text is edited.
func shiftSpans(spans []span, oldText, newText string) []span {
	oldR := []rune(oldText)
	newR := []rune(newText)
	i := 0
	for i < len(oldR) && i < len(newR) && oldR[i] == newR[i] {
		i++
	}
	j := 0
	for j < len(oldR)-i && j < len(newR)-i && oldR[len(oldR)-1-j] == newR[len(newR)-1-j] {
		j++
	}
	oldEnd := len(oldR) - j
	delta := (len(newR) - j) - oldEnd
	var out []span
	for _, s := range spans {
		switch {
		case s.end <= i:
			out = append(out, s)
		case s.start >= oldEnd:
			s.start += delta
			s.end += delta
			if s.end > s.start {
				out = append(out, s)
			}
		default:
			if s.start < i {
				s.end += delta
			} else {
				s.start = i
				s.end += delta
			}
			if s.end < s.start {
				s.end = s.start
			}
			if s.end > len(newR) {
				s.end = len(newR)
			}
			if s.end > s.start {
				out = append(out, s)
			}
		}
	}
	return out
}
