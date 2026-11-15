package layout

import (
	"math"
	"testing"

	"lonchan/internal/model"
)

func TestDefaultRatios(t *testing.T) {
	n := Default("s", "c", "i")
	if math.Abs(n.Ratio-0.10) > 1e-12 {
		t.Fatalf("outer ratio %v", n.Ratio)
	}
	if math.Abs(n.Trailing.Ratio-0.8/0.9) > 1e-12 {
		t.Fatalf("inner ratio %v", n.Trailing.Ratio)
	}
	panes := Panes(n)
	if len(panes) != 3 {
		t.Fatalf("panes %d", len(panes))
	}
	if !panes[0].ShowHeader() || panes[1].ContextID != "c" {
		t.Fatalf("pane bindings %+v %+v", panes[0], panes[1])
	}
}

func TestWidestConversation(t *testing.T) {
	n := Default("s", "c", "i")
	kind := func(id string) string {
		if id == "c" || id == "i" {
			return "conversation"
		}
		return "select"
	}
	if got := WidestConversation(n, kind); got != "pane-chat" {
		t.Fatalf("widest %s", got)
	}
}

func TestSplitPane(t *testing.T) {
	root := Default("s", "c", "i")
	if !SplitPane(root, "pane-chat", &model.Node{Kind: "pane", ID: "pane-new", ContextID: "c2"}, "v") {
		t.Fatal("pane not found")
	}
	panes := Panes(root)
	if len(panes) != 4 || panes[1].ID != "pane-chat" || panes[2].ID != "pane-new" || panes[3].ID != "pane-panes" {
		t.Fatalf("order %v", panes)
	}
	if FindPane(root, "pane-chat").ContextID != "c" {
		t.Fatal("split pane lost its context")
	}
	if SplitPane(root, "missing", &model.Node{Kind: "pane"}, "h") {
		t.Fatal("split a missing pane")
	}
}
