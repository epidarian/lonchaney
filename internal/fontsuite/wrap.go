package fontsuite

import "lonchan/internal/caps"

// Wrap inserts markdown markers around the rune range [start, end).
// The returned indexes are the new selection, or the caret between an empty pair.
func Wrap(text string, start, end int, mark caps.Mark) (string, int, int) {
	rs := []rune(text)
	if start < 0 {
		start = 0
	}
	if end > len(rs) {
		end = len(rs)
	}
	if start > end {
		start, end = end, start
	}
	open, close := markers(mark)
	selected := string(rs[start:end])
	var mid string
	switch mark {
	case caps.Link:
		if selected == "" {
			mid = "text"
		} else {
			mid = selected
		}
		open, close = "[", "](url)"
		selected = mid
	case caps.Heading, caps.Quote, caps.List:
		prefix := open
		open, close = "", ""
		lineStart := start
		for lineStart > 0 && rs[lineStart-1] != '\n' {
			lineStart--
		}
		inserted := []rune(prefix)
		out := append([]rune{}, rs[:lineStart]...)
		out = append(out, inserted...)
		out = append(out, rs[lineStart:]...)
		shift := len(inserted)
		return string(out), start + shift, end + shift
	default:
		mid = selected
	}
	out := append([]rune{}, rs[:start]...)
	out = append(out, []rune(open)...)
	out = append(out, []rune(mid)...)
	out = append(out, []rune(close)...)
	out = append(out, rs[end:]...)
	if selected == "" && mark != caps.Link {
		caret := start + len([]rune(open))
		return string(out), caret, caret
	}
	return string(out), start, start + len([]rune(open+mid+close))
}

func markers(mark caps.Mark) (string, string) {
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
		return "```\n", "\n```"
	case caps.Heading:
		return "# ", ""
	case caps.Quote:
		return "> ", ""
	case caps.List:
		return "- ", ""
	default:
		return "", ""
	}
}
