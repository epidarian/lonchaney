package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"lonchan/internal/hub"
	"lonchan/internal/layout"
	"lonchan/internal/model"
	"lonchan/internal/store"
)

func newPaneTestUI(t *testing.T) (*UI, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := test.NewApp()
	w := a.NewWindow("t")
	w.Resize(fyne.NewSize(1200, 800))
	u := &UI{app: a, win: w, hub: hub.New(st), scrollOff: map[string]fyne.Position{}}
	u.hub.On = func() {}
	u.ensureActive()
	w.SetContent(u.shellRoot())
	return u, st
}

func TestPaneRules(t *testing.T) {
	u, st := newPaneTestUI(t)
	if u.focused != "pane-chat" {
		t.Fatalf("active pane %q", u.focused)
	}
	id := st.Identities()[0].ID
	a, _ := u.hub.EnsureConversation("svc-local", id, model.TargetGroup, "alpha", "alpha")
	b, _ := u.hub.EnsureConversation("svc-local", id, model.TargetGroup, "beta", "beta")

	// A chat opens in the active pane.
	u.openChat(a.ID)
	if layout.FindPane(st.Config().Layout, "pane-chat").ContextID != a.ID {
		t.Fatal("chat did not open in the active pane")
	}

	// A second chat pane, made active, takes the next chat.
	second := u.splitPane("pane-chat", "h", "")
	if second == "" || !u.isConversationPane(second) {
		t.Fatal("split did not make a chat pane")
	}
	u.activate(second)
	u.openChat(b.ID)
	if layout.FindPane(st.Config().Layout, second).ContextID != b.ID {
		t.Fatal("chat did not open in the newly active pane")
	}

	// Opening a chat that is already showing goes to its pane instead of copying it.
	u.openChat(a.ID)
	if u.focused != "pane-chat" || layout.FindPane(st.Config().Layout, second).ContextID != b.ID {
		t.Fatalf("reopen moved the chat: active %q", u.focused)
	}

	// Numbers follow layout order: selection, chat, new pane, inspector.
	if u.paneNumber("pane-select") != 1 || u.paneNumber("pane-chat") != 2 || u.paneNumber(second) != 3 || u.paneNumber("pane-panes") != 4 {
		t.Fatal("pane numbers out of order")
	}

	// Cycling skips the Selection and Panes panes.
	u.cyclePane(1)
	if u.focused != second {
		t.Fatalf("cycle went to %q", u.focused)
	}
	u.cyclePane(1)
	if u.focused != "pane-chat" {
		t.Fatalf("cycle wrapped to %q", u.focused)
	}

	// Only chat panes become active.
	u.activate("pane-panes")
	if u.focused != "pane-chat" {
		t.Fatal("Panes pane became the active pane")
	}

	// Closing the active pane hands the role to another chat pane.
	u.closePane("pane-chat")
	if layout.FindPane(st.Config().Layout, "pane-chat") != nil || u.focused != second {
		t.Fatalf("close left active %q", u.focused)
	}

	// Empty placeholder chats are reused, not piled up.
	before := len(st.Contexts())
	third := u.splitPane(second, "v", "")
	u.closePane(third)
	u.splitPane(second, "v", "")
	if n := len(st.Contexts()); n > before+1 {
		t.Fatalf("empty chats piled up: %d → %d", before, n)
	}
}

func TestActivateKeepsTyping(t *testing.T) {
	u, _ := newPaneTestUI(t)
	second := u.splitPane("pane-chat", "h", "")
	u.refresh()
	c := u.composers[second]
	if c == nil {
		t.Fatal("no message box in the new pane")
	}
	u.win.Canvas().Focus(c)
	if u.focused != second {
		t.Fatal("typing did not make the pane active")
	}
	if u.composers[second] != c || u.win.Canvas().Focused() != c {
		t.Fatal("activating rebuilt the window and dropped the keyboard")
	}
	// A refresh, like the one an arriving message causes, keeps the keyboard there.
	u.refresh()
	if u.win.Canvas().Focused() != u.composers[second] {
		t.Fatal("refresh dropped the keyboard")
	}
}

func TestPanesPopOut(t *testing.T) {
	u, st := newPaneTestUI(t)
	if p := layout.FindPane(st.Config().Layout, "pane-panes"); p == nil || p.ContextID != store.PanesContextID {
		t.Fatal("the right pane does not show the Panes list by default")
	}
	u.popOutPanes()
	if u.panesWin == nil || u.panesPop == nil || !st.Config().PanesWindow {
		t.Fatal("pop out did not open a window")
	}
	u.popOutPanes() // a second pop out reuses the window
	u.dockPanes()
	if u.panesWin != nil || st.Config().PanesWindow {
		t.Fatal("put back left the window open")
	}
}

func TestNavigatorChoices(t *testing.T) {
	u, st := newPaneTestUI(t)
	id := st.Identities()[0].ID
	a, _ := u.hub.EnsureConversation("svc-local", id, model.TargetGroup, "alpha", "alpha")
	u.openChat(a.ID)
	pane := layout.FindPane(st.Config().Layout, "pane-chat")
	var checked []string
	for _, it := range u.showItems(pane) {
		if it.Checked {
			checked = append(checked, it.Label)
		}
	}
	if len(checked) != 1 || checked[0] != "alpha (as "+st.Identities()[0].Label()+")" {
		t.Fatalf("navigator marks %v", checked)
	}
}
