package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/model"
	"lonchan/internal/store"
)

// ChatView is the conversation pane: header, transcript, composer.
type ChatView struct {
	widget.BaseWidget
	ui   *UI
	pane *model.Node
	ctx  model.Context
}

func NewChatView(ui *UI, pane *model.Node, ctx model.Context) *ChatView {
	v := &ChatView{ui: ui, pane: pane, ctx: ctx}
	v.ExtendBaseWidget(v)
	return v
}

// ChatParts returns the header, transcript, and composer for a conversation slot.
func ChatParts(ui *UI, pane *model.Node, ctx model.Context) (fyne.CanvasObject, fyne.CanvasObject, fyne.CanvasObject) {
	v := &ChatView{ui: ui, pane: pane, ctx: ctx}
	header := container.New(vcenter{}, ui.paneHeader(pane, ctx))
	if !pane.ShowHeader() {
		header = container.NewStack()
	}
	list := v.transcript()
	if ctx.Target == "" {
		icon := widget.NewIcon(theme.MailComposeIcon())
		msg := widget.NewLabelWithStyle("No chat selected", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		tip := widget.NewLabelWithStyle("Pick one in the Selection pane, or use File → New User / New Group.", fyne.TextAlignCenter, fyne.TextStyle{})
		tip.Importance = widget.LowImportance
		list = container.NewCenter(container.NewVBox(container.NewCenter(height(48, width(48, icon))), msg, tip))
	}
	entry := newComposer(ui, ctx.Draft, func(s string) {
		v.ctx.Draft = s
		_ = ui.hub.Store.PutContext(v.ctx)
	}, func(text string) {
		v.ctx.Draft = text
		v.submitText(text, func() {})
	})
	entry.paneID = pane.ID
	if ui.composers != nil {
		ui.composers[pane.ID] = entry
	}
	send := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() { v.submitText(entry.wireText(), func() { entry.SetText("") }) })
	send.Importance = widget.HighImportance
	bar := formatBar(ui, ctx, entry, send)
	label := "You"
	iconPath := ""
	if id, ok := ui.hub.Store.Identity(ctx.IdentityID); ok {
		label = id.Label()
		iconPath = ui.hub.Store.AbsIcon(id.Icon)
	}
	who := newAvatarSize(label, iconPath, 48, func() { ui.showIdentityPop(ctx) })
	composer := newComposerPanel(entry, bar, who, ui.composerLines, ui.composerBar, func(lines, barH float32) {
		ui.composerLines = lines
		ui.composerBar = barH
		_ = ui.hub.Store.UpdateConfig(func(cfg *model.Config) {
			cfg.ComposerLines = float64(lines)
			cfg.ComposerBar = float64(barH)
		})
	})
	if ctx.Target != "" {
		list = container.NewBorder(peerBanner(ui, ctx), nil, nil, nil, list)
	}
	return header, list, composer
}

func (v *ChatView) CreateRenderer() fyne.WidgetRenderer {
	h, t, c := ChatParts(v.ui, v.pane, v.ctx)
	return widget.NewSimpleRenderer(container.NewBorder(h, c, nil, nil, t))
}

func (v *ChatView) transcript() fyne.CanvasObject {
	msgs, _ := v.ui.hub.Store.ReadLog(v.ctx.ServiceID, v.ctx.Target)
	box := container.NewVBox()
	var prevFrom, prevMin string
	mineKey := ""
	if id, ok := v.ui.hub.Store.Identity(v.ctx.IdentityID); ok {
		mineKey = id.Public
		if mineKey == "" {
			mineKey = id.Label()
		}
	}
	svc, _ := v.ui.hub.Store.Service(v.ctx.ServiceID)
	c := v.ui.hub.Caps(svc.Plugin)
	scroll := container.NewVScroll(box)
	pending, passes, lastW := false, 0, float32(0)
	relayout := func(w float32) {
		if w != lastW {
			lastW, passes = w, 0
		}
		if pending || passes > 3 {
			return
		}
		pending = true
		passes++
		fyne.Do(func() {
			pending = false
			box.Refresh()
			scroll.Refresh()
		})
	}
	for _, msg := range msgs {
		min := ""
		if len(msg.Time) >= 16 {
			min = msg.Time[:16]
		}
		show := !(msg.From == prevFrom && min == prevMin && min != "")
		shown := msg
		shown.From = v.ui.authorLabel(msg.From)
		block := NewMessageBlock(shown, msg.From == mineKey || msg.From == v.ctx.IdentityID, show, c, v.ui.hovered)
		block.relayout = relayout
		box.Add(block)
		prevFrom, prevMin = msg.From, min
	}
	unread := unreadAfter(msgs, v.ctx.LastReadID)
	btn := widget.NewButtonWithIcon(fmt.Sprintf("%d new", unread), theme.MoveDownIcon(), nil)
	btn.Importance = widget.HighImportance
	btn.OnTapped = func() {
		if len(msgs) > 0 {
			v.ctx.LastReadID = msgs[len(msgs)-1].ID
			_ = v.ui.hub.Store.PutContext(v.ctx)
		}
		v.ui.setBottom(v.pane.ID, true)
		scroll.ScrollToBottom()
		btn.Hide()
	}
	if unread == 0 || v.ui.bottom(v.pane.ID) {
		btn.Hide()
	}
	scroll.OnScrolled = func(off fyne.Position) {
		v.ui.scrollOff[v.pane.ID] = off
		atBottom := off.Y+scroll.Size().Height >= box.MinSize().Height-8
		v.ui.setBottom(v.pane.ID, atBottom)
		if atBottom || unread == 0 {
			btn.Hide()
		} else {
			btn.Show()
		}
	}
	if off, ok := v.ui.scrollOff[v.pane.ID]; ok {
		scroll.Offset = off
	}
	// The pill floats at the top of the transcript so it never sits under the format toolbar,
	// which pops up just above the composer.
	pill := container.NewVBox(container.NewPadded(container.NewCenter(btn)))
	return container.NewStack(scroll, pill)
}

func unreadAfter(msgs []model.LogMessage, last string) int {
	if last == "" {
		return len(msgs)
	}
	n := 0
	seen := false
	for _, m := range msgs {
		if !seen {
			if m.ID == last {
				seen = true
			}
			continue
		}
		n++
	}
	if !seen {
		return len(msgs)
	}
	return n
}

func (v *ChatView) submitText(text string, clear func()) {
	if v.ctx.Target == "" || text == "" {
		return
	}
	if err := v.ui.hub.Send(v.ctx, text); err != nil {
		v.ui.hub.Store.SetHandshake(v.ctx.IdentityID, err.Error())
	}
	clear()
	v.ctx.Draft = ""
	msgs, _ := v.ui.hub.Store.ReadLog(v.ctx.ServiceID, v.ctx.Target)
	if len(msgs) > 0 {
		v.ctx.LastReadID = msgs[len(msgs)-1].ID
	}
	_ = v.ui.hub.Store.PutContext(v.ctx)
}

type composer struct {
	widget.Entry
	ui      *UI
	onSend  func(string)
	paneID  string // the pane this message box is in
	plain   string
	spans   []span
	view    *formatView
	focused bool
	pinA    int
	pinB    int
	anchor  int
}

func newComposer(ui *UI, text string, onChanged func(string), onSend func(string)) *composer {
	plain, spans := splitMarks(text)
	c := &composer{ui: ui, onSend: onSend, plain: plain, spans: spans}
	c.MultiLine = true
	c.Wrapping = fyne.TextWrapWord
	c.ExtendBaseWidget(c)
	c.OnChanged = func(s string) {
		if s != c.plain {
			c.spans = shiftSpans(c.spans, c.plain, s)
			c.plain = s
			c.pinA, c.pinB = 0, 0
		}
		if onChanged != nil {
			onChanged(c.wireText())
		}
		c.poke()
	}
	c.SetText(plain)
	c.SetPlaceHolder("Message")
	return c
}

func (c *composer) wireText() string { return wire(c.Text, c.spans) }

func (c *composer) poke() {
	if c.view != nil {
		c.view.Refresh()
	}
}

func (c *composer) FocusGained() {
	c.focused = true
	c.Entry.FocusGained()
	if c.ui != nil {
		c.ui.activate(c.paneID)
		c.ui.markReadFocused()
		c.ui.composer = c
	}
	c.poke()
}

func (c *composer) noteSelection() {
	a, b := entrySelection(c)
	if a != b {
		c.pinA, c.pinB = a, b
	}
}

func (c *composer) MouseDown(e *desktop.MouseEvent) {
	c.Entry.MouseDown(e)
	off := c.CursorTextOffset()
	c.anchor = off
	c.pinA, c.pinB = off, off
	c.poke()
}

func (c *composer) MouseUp(*desktop.MouseEvent) {
	c.poke()
}

func (c *composer) Dragged(e *fyne.DragEvent) {
	if c.view != nil && c.view.Size().Width > 0 && !pointerInside(e.Position, c.view.Size()) {
		c.poke()
		return
	}
	c.Entry.Dragged(e)
	c.pinA, c.pinB = c.anchor, c.CursorTextOffset()
	c.poke()
}

func (c *composer) DragEnd() {
	c.poke()
}

func (c *composer) FocusLost() {
	c.noteSelection()
	c.focused = false
	c.Entry.FocusLost()
	c.poke()
}

func (c *composer) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		chord := formatChord(fyne.KeyReturn, currentMods())
		keys := map[string]string{}
		if c.ui != nil {
			keys = c.ui.hub.Store.Config().Keys
		}
		cfg := model.Config{}
		if c.ui != nil {
			cfg = c.ui.hub.Store.Config()
		}
		if keys["newline"] == "" {
			keys["newline"] = "Shift+Return"
		}
		if keys["send"] == "" {
			keys["send"] = "Return"
		}
		if store.BindingOn(cfg, "newline") && chord == keys["newline"] {
			c.Entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			return
		}
		if store.BindingOn(cfg, "send") && chord == keys["send"] {
			if c.onSend != nil {
				c.onSend(c.wireText())
				c.SetText("")
			}
			return
		}
	}
	c.Entry.TypedKey(ev)
	c.poke()
}

func (v *ChatView) Tapped(*fyne.PointEvent) { v.ui.focusPane(v.pane.ID) }

// composerHeight is four lines of text, padding around the field, and the format row.
func composerHeight() float32 {
	return messageFieldHeight(4) + 36
}

func messageFieldHeight(lines float32) float32 {
	if lines <= 0 {
		lines = 4
	}
	if lines < 2 {
		lines = 2
	}
	return theme.TextSize()*lines + theme.Padding()*6
}

func composerBox(entry fyne.CanvasObject, bar fyne.CanvasObject) fyne.CanvasObject {
	return newComposerPanel(entry, bar, nil, 0, 0, nil)
}

// composerPanel is the message field, the format row, and the two drag edges.
type composerPanel struct {
	widget.BaseWidget
	entry  fyne.CanvasObject
	shown  fyne.CanvasObject
	ink    *formatView
	bar    fyne.CanvasObject
	avatar fyne.CanvasObject
	lines  float32
	barH   float32
	onEnd  func(lines, barH float32)
}

func newComposerPanel(entry, bar, avatar fyne.CanvasObject, lines, barH float32, onEnd func(float32, float32)) *composerPanel {
	p := &composerPanel{entry: entry, bar: bar, avatar: avatar, lines: lines, barH: barH, onEnd: onEnd}
	p.ExtendBaseWidget(p)
	return p
}

func (p *composerPanel) lineCount() float32 {
	if p.lines <= 0 {
		return 4
	}
	if p.lines < 2 {
		return 2
	}
	if p.lines > 16 {
		return 16
	}
	return p.lines
}

func (p *composerPanel) barHeight() float32 {
	h := p.barH
	if h <= 0 {
		h = 36
	}
	if h < 32 {
		h = 32
	}
	if h > 140 {
		h = 140
	}
	return h
}

func (p *composerPanel) CreateRenderer() fyne.WidgetRenderer {
	msg := newEdgeGrip(p, false)
	area := newEdgeGrip(p, true)
	shown := p.entry
	if c, ok := p.entry.(*composer); ok && fyne.CurrentApp() != nil {
		shown = container.NewThemeOverride(c, hideTextTheme{fyne.CurrentApp().Settings().Theme()})
		p.ink = newFormatView(c)
		c.view = p.ink
	}
	p.shown = shown
	objs := []fyne.CanvasObject{shown, p.bar, msg, area}
	if p.ink != nil {
		objs = []fyne.CanvasObject{shown, p.ink, p.bar, msg, area}
	}
	if p.avatar != nil {
		objs = append([]fyne.CanvasObject{p.avatar}, objs...)
	}
	return &composerRenderer{panel: p, msg: msg, area: area, objs: objs}
}

type composerRenderer struct {
	panel *composerPanel
	msg   *edgeGrip
	area  *edgeGrip
	objs  []fyne.CanvasObject
}

func (r *composerRenderer) Destroy()                     {}
func (r *composerRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *composerRenderer) MinSize() fyne.Size {
	w := r.panel.bar.MinSize().Width
	if ew := r.panel.entry.MinSize().Width; ew > w {
		w = ew
	}
	if r.panel.avatar != nil {
		w += r.panel.avatar.MinSize().Width + theme.Padding()
	}
	if w < 40 {
		w = 40
	}
	return fyne.NewSize(w, messageFieldHeight(r.panel.lineCount())+r.panel.barHeight())
}
func (r *composerRenderer) Refresh() {
	r.Layout(r.panel.Size())
	canvas.Refresh(r.panel)
}
func (r *composerRenderer) Layout(size fyne.Size) {
	p := r.panel
	barH := p.barHeight()
	if barH > size.Height-theme.TextSize() {
		barH = size.Height - theme.TextSize()
	}
	if barH < 0 {
		barH = 0
	}
	fieldH := size.Height - barH
	left := float32(0)
	if p.avatar != nil {
		aw := p.avatar.MinSize().Width
		ah := p.avatar.MinSize().Height
		if ah > size.Height {
			ah = size.Height
		}
		p.avatar.Resize(fyne.NewSize(aw, ah))
		p.avatar.Move(fyne.NewPos(0, (size.Height-ah)/2))
		left = aw + theme.Padding()
	}
	innerW := size.Width - left
	if innerW < 0 {
		innerW = 0
	}
	pad := theme.Padding() * 2
	gripH := float32(14)
	entryY := gripH
	entryH := fieldH - gripH*2
	if entryH < 0 {
		entryH = 0
	}
	field := p.entry
	if p.shown != nil {
		field = p.shown
	}
	field.Move(fyne.NewPos(left+pad, entryY))
	field.Resize(fyne.NewSize(max32(innerW-pad*2, 0), entryH))
	if p.ink != nil {
		p.ink.Move(field.Position())
		p.ink.Resize(field.Size())
	}
	bh := p.bar.MinSize().Height
	if bh > barH {
		bh = barH
	}
	p.bar.Resize(fyne.NewSize(innerW, bh))
	p.bar.Move(fyne.NewPos(left, fieldH+(barH-bh)/2))
	r.msg.Move(fyne.NewPos(left, 0))
	r.msg.Resize(fyne.NewSize(innerW, gripH))
	r.area.Move(fyne.NewPos(left, fieldH-gripH))
	r.area.Resize(fyne.NewSize(innerW, gripH))
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// edgeGrip is the drag control on the message box or the format row.
type edgeGrip struct {
	widget.BaseWidget
	panel *composerPanel
	bar   bool
}

func newEdgeGrip(panel *composerPanel, bar bool) *edgeGrip {
	g := &edgeGrip{panel: panel, bar: bar}
	g.ExtendBaseWidget(g)
	return g
}

func (g *edgeGrip) Dragged(e *fyne.DragEvent) {
	if g.bar {
		g.panel.barH = g.panel.barHeight() - e.Dragged.DY
	} else {
		g.panel.lines = g.panel.lineCount() - e.Dragged.DY/theme.TextSize()
	}
	g.panel.lines = g.panel.lineCount()
	g.panel.barH = g.panel.barHeight()
	g.panel.Refresh()
}

func (g *edgeGrip) DragEnd() {
	if g.panel.onEnd != nil {
		g.panel.onEnd(g.panel.lineCount(), g.panel.barHeight())
	}
}

func (g *edgeGrip) Cursor() desktop.Cursor         { return desktop.VResizeCursor }
func (g *edgeGrip) MouseIn(*desktop.MouseEvent)    {}
func (g *edgeGrip) MouseMoved(*desktop.MouseEvent) {}
func (g *edgeGrip) MouseOut()                      {}

func (g *edgeGrip) CreateRenderer() fyne.WidgetRenderer {
	pill := canvas.NewRectangle(fade(theme.ColorForWidget(theme.ColorNameForeground, g), 0x55))
	pill.CornerRadius = 2
	return &gripRenderer{grip: g, pill: pill}
}

type gripRenderer struct {
	grip *edgeGrip
	pill *canvas.Rectangle
}

func (r *gripRenderer) Destroy()                     {}
func (r *gripRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.pill} }
func (r *gripRenderer) MinSize() fyne.Size           { return fyne.NewSize(28, 8) }
func (r *gripRenderer) Refresh() {
	r.pill.FillColor = fade(theme.ColorForWidget(theme.ColorNameForeground, r.grip), 0x55)
	r.pill.Refresh()
}
func (r *gripRenderer) Layout(s fyne.Size) {
	w := float32(36)
	if w > s.Width {
		w = s.Width
	}
	h := float32(4)
	r.pill.Resize(fyne.NewSize(w, h))
	r.pill.Move(fyne.NewPos((s.Width-w)/2, (s.Height-h)/2))
}

type fixedH struct{ h float32 }

func (f fixedH) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(40, f.h)
}
func (f fixedH) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}
