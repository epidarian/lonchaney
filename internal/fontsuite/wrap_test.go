package fontsuite

import (
	"testing"

	"lonchan/internal/caps"
)

func TestWrapSelection(t *testing.T) {
	got, a, b := Wrap("hello", 0, 5, caps.Bold)
	if got != "**hello**" || a != 0 || b != len([]rune(got)) {
		t.Fatalf("%q %d %d", got, a, b)
	}
}

func TestWrapCaret(t *testing.T) {
	got, a, b := Wrap("ab", 1, 1, caps.Italic)
	if got != "a__b" || a != 2 || b != 2 {
		t.Fatalf("%q %d %d", got, a, b)
	}
}

func TestWrapHeading(t *testing.T) {
	got, _, _ := Wrap("title", 1, 1, caps.Heading)
	if got != "# title" {
		t.Fatalf("%q", got)
	}
}
