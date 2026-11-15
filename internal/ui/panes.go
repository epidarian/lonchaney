package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/layout"
	"lonchan/internal/model"
)

// Pane rules, in one place:
//
//   - Panes are numbered left to right, top to bottom. The number shows in each
//     pane's header, in the Panes list, and in the Pane menu.
//   - Exactly one chat pane is active. It has a colored bar along its top.
//     Clicking in a chat pane, or typing in it, makes it active.
//   - The Selection and Inspector panes always work on the active chat pane.
//   - Opening a chat that is already in a pane goes to that pane. Otherwise the
//     chat opens in the active chat pane.

const paneBarHeight = 3

// paneHost wraps one pane: the colored bar on top and the click that activates it.
type paneHost struct {
	widget.BaseWidget
	ui      *UI
	id      string
	chat    bool
	content fyne.CanvasObject
	sel     *SelectView
}

func newPaneHost(u *UI, id string, chat bool, content fyne.CanvasObject) *paneHost {
	h := &paneHost{ui: u, id: id, chat: chat, content: content}
	h.ExtendBaseWidget(h)
	return h
}

// Tapped receives clicks that land on the pane but not on a button or field.
func (h *paneHost) Tapped(*fyne.PointEvent) {
	if h.chat {
		h.ui.activate(h.id)
	}
}

func (h *paneHost) CreateRenderer() fyne.WidgetRenderer {
	r := &paneHostRenderer{h: h, bar: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type paneHostRenderer struct {
	h   *paneHost
	bar *canvas.Rectangle
}

func (r *paneHostRenderer) barH() float32 {
	if r.h.chat {
		return paneBarHeight
	}
	return 0
}

func (r *paneHostRenderer) Destroy() {}
func (r *paneHostRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.h.content, r.bar}
}
func (r *paneHostRenderer) MinSize() fyne.Size {
	return r.h.content.MinSize().AddWidthHeight(0, r.barH())
}
func (r *paneHostRenderer) Layout(s fyne.Size) {
	b := r.barH()
	r.bar.Move(fyne.NewPos(0, 0))
	r.bar.Resize(fyne.NewSize(s.Width, b))
	r.h.content.Move(fyne.NewPos(0, b))
	r.h.content.Resize(fyne.NewSize(s.Width, s.Height-b))
}
func (r *paneHostRenderer) Refresh() {
	r.bar.FillColor = color.Transparent
	if r.h.chat && r.h.ui.focused == r.h.id {
		r.bar.FillColor = theme.ColorForWidget(theme.ColorNamePrimary, r.h)
	}
	r.bar.Refresh()
	r.Layout(r.h.Size())
}

// paneBadge is the pane number in a pane header. It is filled on the active pane.
type paneBadge struct {
	widget.BaseWidget
	ui *UI
	id string
}

func newPaneBadge(u *UI, id string) *paneBadge {
	b := &paneBadge{ui: u, id: id}
	b.ExtendBaseWidget(b)
	u.badges = append(u.badges, b)
	return b
}

func (b *paneBadge) CreateRenderer() fyne.WidgetRenderer {
	r := &paneBadgeRenderer{b: b, bg: canvas.NewRectangle(color.Transparent), text: canvas.NewText("", color.Transparent)}
	r.text.Alignment = fyne.TextAlignCenter
	r.text.TextStyle.Bold = true
	r.Refresh()
	return r
}

type paneBadgeRenderer struct {
	b    *paneBadge
	bg   *canvas.Rectangle
	text *canvas.Text
}

func (r *paneBadgeRenderer) Destroy() {}
func (r *paneBadgeRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.text}
}
func (r *paneBadgeRenderer) side() float32 {
	return r.text.MinSize().Height + 2
}
func (r *paneBadgeRenderer) MinSize() fyne.Size {
	s := r.side()
	w := r.text.MinSize().Width + 10
	if w < s {
		w = s
	}
	return fyne.NewSize(w+theme.SizeForWidget(theme.SizeNamePadding, r.b), s)
}
func (r *paneBadgeRenderer) Layout(sz fyne.Size) {
	s := r.side()
	w := r.text.MinSize().Width + 10
	if w < s {
		w = s
	}
	pos := fyne.NewPos(theme.SizeForWidget(theme.SizeNamePadding, r.b), (sz.Height-s)/2)
	r.bg.Move(pos)
	r.bg.Resize(fyne.NewSize(w, s))
	r.bg.CornerRadius = s / 2
	r.text.Move(pos)
	r.text.Resize(fyne.NewSize(w, s))
}
func (r *paneBadgeRenderer) Refresh() {
	b := r.b
	r.text.Text = itoa(b.ui.paneNumber(b.id))
	r.text.TextSize = theme.SizeForWidget(theme.SizeNameCaptionText, b)
	if b.ui.focused == b.id && b.ui.isConversationPane(b.id) {
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNamePrimary, b)
		r.text.Color = theme.ColorForWidget(theme.ColorNameForegroundOnPrimary, b)
	} else {
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNameHover, b)
		r.text.Color = theme.ColorForWidget(theme.ColorNameForeground, b)
	}
	r.bg.Refresh()
	r.text.Refresh()
}

// paneNumber is a pane's 1-based position in the layout, or 0.
func (u *UI) paneNumber(id string) int {
	for i, p := range layout.Panes(u.hub.Store.Config().Layout) {
		if p.ID == id {
			return i + 1
		}
	}
	return 0
}

// paneTitle is how a pane is named in lists and menus: "Pane 2 · bridge".
func (u *UI) paneTitle(p *model.Node) string {
	name := "empty"
	if ctx, ok := u.hub.Store.Context(p.ContextID); ok {
		switch {
		case ctx.Kind == model.KindConversation && ctx.Target != "":
			name = ctx.Target
		case ctx.Kind == model.KindConversation:
			name = "no chat"
		case ctx.Name != "":
			name = ctx.Name
		}
	}
	return "Pane " + itoa(u.paneNumber(p.ID)) + " · " + name
}

// chatPanes are the panes showing a conversation, in layout order.
func (u *UI) chatPanes() []*model.Node {
	var out []*model.Node
	for _, p := range layout.Panes(u.hub.Store.Config().Layout) {
		if u.isConversationPane(p.ID) {
			out = append(out, p)
		}
	}
	return out
}

// activate makes a chat pane the active one without rebuilding the window,
// so whatever the user is typing keeps its focus.
func (u *UI) activate(id string) {
	if id == u.focused || !u.isConversationPane(id) {
		return
	}
	u.focusPane(id)
	for _, h := range u.hosts {
		h.Refresh()
		if h.sel != nil {
			h.sel.Refresh()
		}
	}
	for _, b := range u.badges {
		b.Refresh()
	}
	if u.panesPop != nil {
		u.panesPop.Refresh()
	}
	for _, h := range u.hosts {
		if !h.chat && h.sel == nil {
			// The Inspector describes the active chat; rebuild it.
			if p := layout.FindPane(u.hub.Store.Config().Layout, h.id); p != nil {
				h.content = u.buildPane(p)
				h.Refresh()
				h.BaseWidget.Refresh()
			}
		}
	}
	u.setMenu()
}

// ensureActive keeps u.focused on a chat pane when there is one.
func (u *UI) ensureActive() {
	if u.isConversationPane(u.focused) {
		return
	}
	if panes := u.chatPanes(); len(panes) > 0 {
		u.focused = panes[0].ID
	}
}

// openChat shows a conversation. A pane already showing it becomes active;
// otherwise it replaces the chat in the active pane.
func (u *UI) openChat(contextID string) {
	for _, p := range layout.Panes(u.hub.Store.Config().Layout) {
		if p.ContextID == contextID {
			u.focusPane(p.ID)
			u.refresh()
			u.focusComposer(p.ID)
			return
		}
	}
	u.openChatIn(u.destinationPane(), contextID)
}

// openChatIn shows a conversation in a given pane, even if another pane shows it too.
func (u *UI) openChatIn(paneID, contextID string) {
	u.mountInPane(paneID, contextID)
	u.focusPane(paneID)
	u.refresh()
	u.focusComposer(paneID)
}

// openChatInNewPane splits the active chat pane and opens the conversation in the new half.
func (u *UI) openChatInNewPane(contextID, dir string) {
	if id := u.splitPane(u.destinationPane(), dir, contextID); id != "" {
		u.focusPane(id)
		u.refresh()
		u.focusComposer(id)
	}
}

// splitPane divides a pane. The new pane shows contextID, or an empty chat when
// contextID is "". It returns the new pane's id.
func (u *UI) splitPane(paneID, dir, contextID string) string {
	if contextID == "" {
		contextID = u.emptyChat()
	}
	cfg := u.hub.Store.Config()
	id := "pane-" + modelID()
	if !layout.SplitPane(cfg.Layout, paneID, &model.Node{Kind: "pane", ID: id, ContextID: contextID, HeaderVisible: model.BoolPtr(true)}, dir) {
		return ""
	}
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Layout = cfg.Layout })
	return id
}

// closePane removes a pane. Its neighbor takes the space.
func (u *UI) closePane(paneID string) {
	cfg := u.hub.Store.Config()
	if len(layout.Panes(cfg.Layout)) < 2 {
		return
	}
	cfg.Layout = layout.RemovePane(cfg.Layout, paneID)
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Layout = cfg.Layout })
	if u.focused == paneID {
		u.focused = ""
		u.ensureActive()
		if u.focused != "" {
			u.focusPane(u.focused)
		}
	}
	if u.inSettings {
		u.reopenSettings()
		return
	}
	u.refresh()
}

// emptyChat is a conversation context with no chat in it that no pane shows,
// made if none exists. New panes start on it.
func (u *UI) emptyChat() string {
	shown := map[string]bool{}
	for _, p := range layout.Panes(u.hub.Store.Config().Layout) {
		shown[p.ContextID] = true
	}
	for _, c := range u.hub.Store.Contexts() {
		if c.Kind == model.KindConversation && c.Target == "" && !shown[c.ID] {
			return c.ID
		}
	}
	ctx := model.Context{ID: "ctx-" + modelID(), Name: "Chat", Kind: model.KindConversation}
	if ids := u.hub.Store.Identities(); len(ids) > 0 {
		ctx.IdentityID = ids[0].ID
	}
	if svcs := u.hub.Store.Services(); len(svcs) > 0 {
		ctx.ServiceID = svcs[0].ID
	}
	_ = u.hub.Store.PutContext(ctx)
	return ctx.ID
}

// focusComposer puts the keyboard in a chat pane's message box.
func (u *UI) focusComposer(paneID string) {
	if c := u.composers[paneID]; c != nil && u.win != nil {
		u.win.Canvas().Focus(c)
	}
}

// openChatItems are the right-click choices for a chat in the Selection pane.
func (u *UI) openChatItems(contextID string) []*fyne.MenuItem {
	var items []*fyne.MenuItem
	for _, p := range u.chatPanes() {
		pane := p
		it := fyne.NewMenuItem("Open in "+u.paneTitle(pane), func() { u.openChatIn(pane.ID, contextID) })
		it.Checked = pane.ContextID == contextID
		items = append(items, it)
	}
	if len(items) > 0 {
		items = append(items, fyne.NewMenuItemSeparator())
	}
	return append(items,
		fyne.NewMenuItem("Open in New Pane to the Right", func() { u.openChatInNewPane(contextID, "h") }),
		fyne.NewMenuItem("Open in New Pane Below", func() { u.openChatInNewPane(contextID, "v") }),
	)
}

// navTitle is a pane header's title. Clicking or right-clicking it opens the navigator: the
// list of chats and views this pane can switch to, then the pane's own actions.
type navTitle struct {
	widget.BaseWidget
	ui      *UI
	pane    *model.Node
	text    fyne.CanvasObject
	natural float32 // the full width of the title text
	hovered bool
}

func newNavTitle(u *UI, pane *model.Node, text fyne.CanvasObject) *navTitle {
	n := &navTitle{ui: u, pane: pane, text: text}
	n.ExtendBaseWidget(n)
	return n
}

func (n *navTitle) Tapped(*fyne.PointEvent) {
	n.ui.activate(n.pane.ID)
	n.ui.showMenuBelow(fyne.NewMenu("", n.ui.navItems(n.pane)...), n)
}

func (n *navTitle) MouseIn(*desktop.MouseEvent) {
	n.hovered = true
	n.Refresh()
}
func (n *navTitle) MouseMoved(*desktop.MouseEvent) {}
func (n *navTitle) MouseOut() {
	n.hovered = false
	n.Refresh()
}
func (n *navTitle) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (n *navTitle) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	arrow := widget.NewIcon(theme.MenuDropDownIcon())
	r := &navTitleRenderer{n: n, bg: bg, arrow: arrow}
	r.Refresh()
	return r
}

type navTitleRenderer struct {
	n     *navTitle
	bg    *canvas.Rectangle
	arrow *widget.Icon
}

func (r *navTitleRenderer) Destroy() {}
func (r *navTitleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.n.text, r.arrow}
}
func (r *navTitleRenderer) iconSize() float32 {
	return theme.SizeForWidget(theme.SizeNameInlineIcon, r.n)
}
func (r *navTitleRenderer) MinSize() fyne.Size {
	t := r.n.text.MinSize()
	return fyne.NewSize(r.iconSize()*2, fyne.Max(t.Height, r.iconSize()))
}
func (r *navTitleRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	icon := r.iconSize()
	// The arrow sits right after the text, or at the right edge when the text is cut short.
	tw := fyne.Min(r.n.natural, s.Width-icon)
	r.n.text.Move(fyne.NewPos(0, 0))
	r.n.text.Resize(fyne.NewSize(s.Width-icon, s.Height))
	r.arrow.Move(fyne.NewPos(tw, (s.Height-icon)/2))
	r.arrow.Resize(fyne.NewSize(icon, icon))
}
func (r *navTitleRenderer) Refresh() {
	r.bg.FillColor = color.Transparent
	if r.n.hovered {
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNameHover, r.n)
	}
	r.bg.CornerRadius = theme.SizeForWidget(theme.SizeNameSelectionRadius, r.n)
	r.bg.Refresh()
	r.n.text.Refresh()
}

// TappedSecondary opens the same menu as a click.
func (n *navTitle) TappedSecondary(*fyne.PointEvent) { n.Tapped(nil) }
