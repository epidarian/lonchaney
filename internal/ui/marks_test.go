package ui

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestMarksRoundTrip(t *testing.T) {
	plain, spans := splitMarks("say **hi** _there_")
	if plain != "say hi there" {
		t.Fatalf("plain %q", plain)
	}
	if wire(plain, spans) != "say **hi** _there_" {
		t.Fatalf("wire %q", wire(plain, spans))
	}
	next := shiftSpans(spans, plain, "say hi there!")
	if wire("say hi there!", next) != "say **hi** _there_!" {
		t.Fatalf("shifted %q", wire("say hi there!", next))
	}
}

func TestPointerInside(t *testing.T) {
	box := fyne.NewSize(100, 40)
	if !pointerInside(fyne.NewPos(0, 0), box) || !pointerInside(fyne.NewPos(100, 40), box) {
		t.Fatal("edges")
	}
	if pointerInside(fyne.NewPos(-0.1, 10), box) || pointerInside(fyne.NewPos(10, 41), box) {
		t.Fatal("outside")
	}
}
