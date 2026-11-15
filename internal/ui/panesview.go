package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/layout"
	"lonchan/internal/model"
	"lonchan/internal/skin"
)

// PanesView lists the app's windows and the panes in each. It is a pane kind of
// its own, and it can be popped out into a separate window.
type PanesView struct {
	widget.BaseWidget
	ui   *UI
	pane *model.Node // nil when the view is the content of the separate window
	root *fyne.Container
}

func NewPanesView(u *UI, pane *model.Node) *PanesView {
	v := &PanesView{ui: u, pane: pane, root: container.NewStack()}
	v.ExtendBaseWidget(v)
	return v
}

func (v *PanesView) CreateRenderer() fyne.WidgetRenderer {
	v.fill()
	return widget.NewSimpleRenderer(v.root)
}

// Refresh rebuilds the list from the current layout.
func (v *PanesView) Refresh() {
	v.fill()
	v.BaseWidget.Refresh()
}

func (v *PanesView) fill() {
	v.root.Objects = []fyne.CanvasObject{keepScope(v, v.page())}
	v.root.Refresh()
}

func (v *PanesView) page() fyne.CanvasObject {
	u := v.ui
	var top fyne.CanvasObject = container.NewStack()
	if v.pane != nil && v.pane.ShowHeader() {
		ctx, _ := u.hub.Store.Context(v.pane.ContextID)
		top = u.paneHeader(v.pane, ctx)
	}

	// In the main window while the list is popped out: point to it instead.
	if v.pane != nil && u.panesWin != nil {
		back := widget.NewButtonWithIcon("Bring Back", theme.WindowMinimizeIcon(), u.dockPanes)
		show := widget.NewButtonWithIcon("Show Window", theme.WindowMaximizeIcon(), func() { u.panesWin.RequestFocus() })
		body := container.NewVBox(hint("Popped out."), show, back)
		return container.NewBorder(top, nil, nil, nil, container.NewPadded(body))
	}

	var action *widget.Button
	if v.pane != nil {
		action = widget.NewButtonWithIcon("Pop Out", theme.WindowMaximizeIcon(), u.popOutPanes)
	} else {
		action = widget.NewButtonWithIcon("Put Back", theme.WindowMinimizeIcon(), u.dockPanes)
	}
	action.Importance = widget.LowImportance
	action.Alignment = widget.ButtonAlignLeading

	list := container.NewVBox(boldLabel("Main window"))
	for _, p := range layout.Panes(u.hub.Store.Config().Layout) {
		pane := p
		chat := u.isConversationPane(pane.ID)
		sub := "tool"
		switch {
		case v.pane != nil && pane.ID == v.pane.ID:
			sub = "this list"
		case chat && pane.ID == u.focused:
			sub = "chat · active"
		case chat:
			sub = "chat"
		}
		var tap func()
		if chat {
			tap = func() { u.activate(pane.ID) }
		}
		line := newChatLine(u.paneTitle(pane), sub, 0, chat && pane.ID == u.focused, false, tap)
		line.menu = func() []*fyne.MenuItem { return u.paneItems(pane) }
		list.Add(line)
	}
	if u.panesWin != nil {
		list.Add(boldLabel("Separate window"))
		list.Add(newChatLine("Panes", "this list · close it to put it back", 0, false, false, nil))
	}
	list.Add(hint("Click a chat pane to make it active. Right-click any pane for its menu."))
	return container.NewBorder(top, nil, nil, nil, container.NewVScroll(container.NewVBox(action, list)))
}

// popOutPanes moves the Panes list into its own window.
func (u *UI) popOutPanes() {
	if u.panesWin != nil {
		u.panesWin.RequestFocus()
		return
	}
	w := u.app.NewWindow("Panes — lonchan")
	v := NewPanesView(u, nil)
	pack, dir := u.activePack()
	w.SetContent(u.newFrame(pack, dir, panesSlot(pack), nil, v))
	w.Resize(fyne.NewSize(300, 480))
	w.SetCloseIntercept(u.dockPanes)
	u.panesWin, u.panesPop = w, v
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.PanesWindow = true })
	w.Show()
	u.refresh()
}

// dockPanes closes the separate Panes window; the list shows in its pane again.
func (u *UI) dockPanes() {
	w := u.panesWin
	if w == nil {
		return
	}
	u.panesWin, u.panesPop = nil, nil
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.PanesWindow = false })
	w.Close()
	u.refresh()
}

// panesSlot is the skin slot the Panes list is drawn with: "panes", or the
// "inspector" drawing for skins made before the Panes list had its own slot.
func panesSlot(pack skin.Pack) string {
	if _, ok := pack.Panes["panes"]; !ok {
		if _, ok := pack.Panes["inspector"]; ok {
			return "inspector"
		}
	}
	return "panes"
}
