package xstyle

import (
	"strings"

	"mellium.im/xmpp/styling"
)

// ToXEP converts a markdown subset into XEP-0393 styling.
func ToXEP(markdown string) string {
	s := markdown
	s = swapPair(s, "**", "*")
	s = swapPair(s, "~~", "~")
	return s
}

// Marks reports which suite marks a styled body uses.
func Marks(body string) []string {
	dec := styling.NewDecoder(strings.NewReader(body))
	found := map[string]bool{}
	for dec.Next() {
		_ = dec.Token()
		st := dec.Style()
		if st&styling.SpanStrong != 0 {
			found["bold"] = true
		}
		if st&styling.SpanEmph != 0 {
			found["italic"] = true
		}
		if st&styling.SpanStrike != 0 {
			found["strike"] = true
		}
		if st&styling.SpanPre != 0 {
			found["code"] = true
		}
		if st&styling.BlockPre != 0 {
			found["code_block"] = true
		}
		if st&styling.BlockQuote != 0 {
			found["quote"] = true
		}
	}
	order := []string{"bold", "italic", "strike", "code", "code_block", "quote"}
	var out []string
	for _, name := range order {
		if found[name] {
			out = append(out, name)
		}
	}
	return out
}

func swapPair(s, from, to string) string {
	parts := strings.Split(s, from)
	if len(parts) < 3 {
		return s
	}
	var b strings.Builder
	for i, part := range parts {
		if i%2 == 1 && i != len(parts)-1 {
			b.WriteString(to)
			b.WriteString(part)
			b.WriteString(to)
			continue
		}
		b.WriteString(part)
	}
	return b.String()
}
