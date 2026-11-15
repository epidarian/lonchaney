package ui

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/caps"
	"lonchan/internal/hub"
	"lonchan/internal/layout"
	"lonchan/internal/model"
	"lonchan/internal/skin"
	"lonchan/internal/store"
)

// UI is the Fyne shell.
type UI struct {
	app             fyne.App
	win             fyne.Window
	hub             *hub.Hub
	hovered         caps.Mark
	inSettings      bool
	scrollOff       map[string]fyne.Position
	recording       string
	focused         string
	composer        *composer
	suite           *widget.PopUp
	atBottom        map[string]bool
	layoutPick      string
	contextPick     string
	identityPick    string
	settingsSection string
	settingsOff     map[string]fyne.Position
	hosts           []*paneHost
	badges          []*paneBadge
	composers       map[string]*composer
	panesWin        fyne.Window // the separate Panes window, or nil
	panesPop        *PanesView  // the list inside it
	composerLines   float32
	composerBar     float32
}

// Run opens the window on st and blocks until it closes.
func Run(st *store.Store) {
	a := app.New()
	u := &UI{app: a, hub: hub.New(st), scrollOff: map[string]fyne.Position{}}
	u.composerLines = float32(st.Config().ComposerLines)
	u.composerBar = float32(st.Config().ComposerBar)
	a.Settings().SetTheme(u.buildTheme())
	u.win = a.NewWindow("lonchan")
	u.win.Resize(fyne.NewSize(1200, 800))
	u.hub.On = func() {
		fyne.Do(func() {
			if !u.inSettings {
				u.refresh()
			}
		})
	}
	for _, svc := range st.Services() {
		if svc.Plugin == model.PluginLocal {
			_ = u.hub.Connect(svc)
		}
	}
	u.focused = st.Config().FocusedPane
	u.ensureActive()
	u.win.SetContent(u.shellRoot())
	u.setMenu()
	u.win.Canvas().SetOnTypedKey(u.onKey)
	u.bindShortcuts()
	u.win.SetMaster()
	u.win.SetCloseIntercept(func() {
		u.hub.Close()
		if u.panesWin != nil {
			// Closing the app with Panes popped out keeps it popped out next time.
			u.panesWin.Close()
		}
		u.win.Close()
	})
	if st.Config().PanesWindow {
		u.popOutPanes()
	}
	u.win.ShowAndRun()
}

func (u *UI) refresh() {
	if u.inSettings {
		return
	}
	// Rebuilding replaces every control. Put the keyboard and caret back in the
	// message box that had them, so an arriving message does not interrupt typing.
	keep, row, col := "", 0, 0
	if c, ok := u.win.Canvas().Focused().(*composer); ok {
		keep, row, col = c.paneID, c.CursorRow, c.CursorColumn
	}
	u.win.SetContent(u.shellRoot())
	u.retitle()
	u.setMenu()
	if u.panesPop != nil {
		u.panesPop.Refresh()
	}
	if c := u.composers[keep]; keep != "" && c != nil {
		u.win.Canvas().Focus(c)
		c.CursorRow, c.CursorColumn = row, col
		c.Refresh()
	}
}

func (u *UI) applyTheme() {
	u.app.Settings().SetTheme(u.buildTheme())
}

func (u *UI) retitle() {
	ctx := u.focusedConversation()
	if ctx.ID != "" && ctx.Target != "" {
		u.win.SetTitle("lonchan — " + ctx.Target)
		return
	}
	u.win.SetTitle("lonchan")
}

func (u *UI) shellRoot() fyne.CanvasObject {
	cfg := u.hub.Store.Config()
	tree := cfg.Layout
	if tree == nil {
		tree = layout.Default("ctx-select", "ctx-chat", store.PanesContextID)
	}
	u.hosts, u.badges, u.composers = nil, nil, map[string]*composer{}
	u.ensureActive()
	pack, dir := u.activePack()
	return u.newFrame(pack, dir, "window", nil, u.build(tree, pack, dir, float32(cfg.SplitHandlePx)))
}

func (u *UI) activePack() (skin.Pack, string) {
	name := u.hub.Store.Config().Skin
	if name == "" || name == "plain" {
		return skin.Plain(), ""
	}
	dir := filepath.Join(u.hub.Store.Dir(), "skins", name)
	loaded, err := skin.Load(dir)
	if err != nil {
		return skin.Plain(), ""
	}
	return loaded.Pack, loaded.Dir
}

func (u *UI) newFrame(pack skin.Pack, dir, slot string, parts map[string]fyne.CanvasObject, fallback fyne.CanvasObject) *Frame {
	f := NewFrame(pack, slot, parts, fallback)
	f.Dir = dir
	cfg := u.hub.Store.Config()
	if cfg.IgnoreSkinTheme {
		return f
	}
	// A pane that names a different skin takes that skin's theme block too;
	// the window skin's block is already the app theme.
	var over *skin.Theme
	if _, activeDir := u.activePack(); dir != activeDir {
		over = pack.Theme
	}
	over = mergeTheme(over, pack.Panes[slot].Theme)
	if over != nil {
		f.Theme = paneTheme(over, pack, dir, cfg)
	}
	return f
}

// mergeTheme lays b over a. Either may be nil.
func mergeTheme(a, b *skin.Theme) *skin.Theme {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := &skin.Theme{Variant: a.Variant}
	if b.Variant != "" {
		out.Variant = b.Variant
	}
	strs := func(x, y map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range x {
			m[k] = v
		}
		for k, v := range y {
			m[k] = v
		}
		return m
	}
	out.Colors = strs(a.Colors, b.Colors)
	out.Fonts = strs(a.Fonts, b.Fonts)
	out.Icons = strs(a.Icons, b.Icons)
	out.Sizes = map[string]float64{}
	for k, v := range a.Sizes {
		out.Sizes[k] = v
	}
	for k, v := range b.Sizes {
		out.Sizes[k] = v
	}
	return out
}

func (u *UI) build(n *model.Node, pack skin.Pack, dir string, handle float32) fyne.CanvasObject {
	if n == nil {
		return widget.NewLabel("")
	}
	if n.Kind == "split" {
		sp := NewSplit(n.Dir != "v", n.Ratio, handle, u.build(n.Leading, pack, dir, handle), u.build(n.Trailing, pack, dir, handle), func(r float64) {
			n.Ratio = r
			_ = u.hub.Store.UpdateConfig(func(*model.Config) {})
		})
		// The separator strip stands in for a handle only when the skin draws none.
		var plainHandle fyne.CanvasObject = canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
		if len(pack.Panes["handle"].Layers) > 0 || skin.File(dir, "handle") != "" {
			plainHandle = nil
		}
		sp.Decor = u.newFrame(pack, dir, "handle", nil, plainHandle)
		return sp
	}
	obj, sel := u.paneContent(n, pack, dir)
	h := newPaneHost(u, n.ID, u.isConversationPane(n.ID), obj)
	h.sel = sel
	u.hosts = append(u.hosts, h)
	return h
}

// buildPane rebuilds one pane's content with the window skin.
func (u *UI) buildPane(n *model.Node) fyne.CanvasObject {
	pack, dir := u.activePack()
	obj, _ := u.paneContent(n, pack, dir)
	return obj
}

// paneContent is a pane's skinned frame and, for a Selection pane, its view.
func (u *UI) paneContent(n *model.Node, pack skin.Pack, dir string) (fyne.CanvasObject, *SelectView) {
	ctx, _ := u.hub.Store.Context(n.ContextID)
	slot := "conversation"
	switch ctx.Kind {
	case model.KindSelect:
		slot = "select"
	case model.KindInspector:
		slot = "inspector"
	}
	use, useDir := pack, dir
	if n.SkinID != "" && n.SkinID != "plain" {
		if loaded, err := skin.Load(filepath.Join(u.hub.Store.Dir(), "skins", n.SkinID)); err == nil {
			use = loaded.Pack
			useDir = loaded.Dir
		}
	}
	switch ctx.Kind {
	case model.KindSelect:
		sel := NewSelectView(u, n)
		return u.newFrame(use, useDir, slot, nil, sel), sel
	case model.KindPanes:
		return u.newFrame(use, useDir, panesSlot(use), nil, NewPanesView(u, n)), nil
	case model.KindInspector:
		body := InspectorView(u, ctx)
		var top fyne.CanvasObject
		if n.ShowHeader() {
			top = u.paneHeader(n, ctx)
		}
		return u.newFrame(use, useDir, slot, nil, container.NewBorder(top, nil, nil, nil, body)), nil
	default:
		header, transcript, composer := ChatParts(u, n, ctx)
		fallback := container.NewBorder(header, composer, nil, nil, transcript)
		return u.newFrame(use, useDir, slot, map[string]fyne.CanvasObject{
			"header": header, "transcript": transcript, "composer": composer,
		}, fallback), nil
	}
}

// paneHeader is a single row: a truncating title and a menu button that holds
// the pane's context and identity choices.
func (u *UI) paneHeader(pane *model.Node, ctx model.Context) fyne.CanvasObject {
	identLabel := ""
	if id, ok := u.hub.Store.Identity(ctx.IdentityID); ok {
		identLabel = id.Label()
	} else if ids := u.hub.Store.Identities(); len(ids) > 0 {
		identLabel = ids[0].Label()
	}
	title, sub := ctx.Name, ""
	if ctx.Kind == model.KindConversation {
		title = ctx.Target
		if title == "" {
			title = "No chat selected"
		}
		sub = ctx.Name
		if identLabel != "" {
			sub += " · " + identLabel
		}
	}
	if title == "" {
		title = "Pane"
	}
	grey := false
	if ctx.Kind == model.KindConversation {
		svc, _ := u.hub.Store.Service(ctx.ServiceID)
		grey = caps.HeaderGrey(u.hub.Caps(svc.Plugin), u.hovered)
	}
	if grey {
		label := truncLabel(strings.TrimSpace(title + "   " + sub))
		label.Importance = widget.LowImportance
		return container.NewBorder(nil, nil, newPaneBadge(u, pane.ID), nil, label)
	}
	segs := []widget.RichTextSegment{&widget.TextSegment{Text: title, Style: widget.RichTextStyleStrong}}
	size := theme.TextSize()
	natural := theme.InnerPadding()*2 + fyne.MeasureText(title, size, fyne.TextStyle{Bold: true}).Width
	if sub != "" {
		segs = append(segs, &widget.TextSegment{Text: "   " + sub, Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder}})
		natural += fyne.MeasureText("   "+sub, size, fyne.TextStyle{}).Width
	}
	rt := widget.NewRichText(segs...)
	rt.Truncation = fyne.TextTruncateEllipsis
	nav := newNavTitle(u, pane, rt)
	nav.natural = natural
	return container.NewBorder(nil, nil, newPaneBadge(u, pane.ID), nil, nav)
}

// navItems is the menu under a pane's title: what the pane can show, then the
// pane's own actions.
func (u *UI) navItems(pane *model.Node) []*fyne.MenuItem {
	items := u.showItems(pane)
	rest := u.paneItems(pane)
	if len(rest) > 1 && rest[1].IsSeparator {
		rest = rest[1:]
	}
	if len(rest) > 1 {
		items = append(append(items, fyne.NewMenuItemSeparator()), rest[1:]...)
	}
	return items
}

// setIdentity switches ctx to ident, moving to a matching IRC/XMPP service when needed.
func (u *UI) setIdentity(ctx model.Context, identID string) {
	ctx.IdentityID = identID
	if svc, ok := u.hub.Store.Service(ctx.ServiceID); ok && (svc.Plugin == model.PluginIRC || svc.Plugin == model.PluginXMPP) {
		for _, other := range u.hub.Store.Services() {
			if other.Plugin == svc.Plugin && other.IdentityID == identID {
				ctx.ServiceID = other.ID
				break
			}
		}
	}
	_ = u.hub.Store.PutContext(ctx)
	u.refresh()
}

func (u *UI) focusPane(id string) {
	u.focused = id
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.FocusedPane = id })
	u.markReadFocused()
	u.retitle()
}

func (u *UI) markReadFocused() {
	ctx := u.focusedConversation()
	if ctx.ID == "" || ctx.Target == "" {
		return
	}
	msgs, err := u.hub.Store.ReadLog(ctx.ServiceID, ctx.Target)
	if err != nil || len(msgs) == 0 {
		return
	}
	latest := msgs[len(msgs)-1].ID
	if ctx.LastReadID == latest {
		return
	}
	ctx.LastReadID = latest
	_ = u.hub.Store.PutContext(ctx)
}

func (u *UI) bottom(id string) bool {
	if u.atBottom == nil {
		return true
	}
	v, ok := u.atBottom[id]
	if !ok {
		return true
	}
	return v
}

func (u *UI) setBottom(id string, v bool) {
	if u.atBottom == nil {
		u.atBottom = map[string]bool{}
	}
	u.atBottom[id] = v
}

func currentMods() fyne.KeyModifier {
	if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
		return d.CurrentKeyModifiers()
	}
	return 0
}

func formatChord(key fyne.KeyName, mod fyne.KeyModifier) string {
	var b strings.Builder
	if mod&fyne.KeyModifierControl != 0 {
		b.WriteString("Ctrl+")
	}
	if mod&fyne.KeyModifierAlt != 0 {
		b.WriteString("Alt+")
	}
	if mod&fyne.KeyModifierShift != 0 {
		b.WriteString("Shift+")
	}
	if mod&fyne.KeyModifierSuper != 0 {
		b.WriteString("Super+")
	}
	b.WriteString(keyToken(key))
	return b.String()
}

func keyToken(key fyne.KeyName) string {
	switch key {
	case fyne.KeyReturn, fyne.KeyEnter:
		return "Return"
	case fyne.KeyComma:
		return ","
	case fyne.KeyTab:
		return "Tab"
	case fyne.KeyEscape:
		return "Escape"
	default:
		return string(key)
	}
}

func (u *UI) focusedConversation() model.Context {
	cfg := u.hub.Store.Config()
	if p := layout.FindPane(cfg.Layout, u.focused); p != nil {
		if ctx, ok := u.hub.Store.Context(p.ContextID); ok && ctx.Kind == model.KindConversation {
			return ctx
		}
	}
	for _, ctx := range u.hub.Store.Contexts() {
		if ctx.Kind == model.KindConversation {
			return ctx
		}
	}
	return model.Context{}
}

// mountConversation opens a chat the way clicking it in the Selection pane does.
func (u *UI) mountConversation(contextID string) {
	u.openChat(contextID)
}

// destinationPane is where a chat opens: the active chat pane, else the first
// chat pane, else the first pane that is not a Selection list.
func (u *UI) destinationPane() string {
	if u.isConversationPane(u.focused) {
		return u.focused
	}
	panes := layout.Panes(u.hub.Store.Config().Layout)
	for _, p := range panes {
		if u.isConversationPane(p.ID) {
			return p.ID
		}
	}
	for _, p := range panes {
		if ctx, ok := u.hub.Store.Context(p.ContextID); !ok || ctx.Kind != model.KindSelect {
			return p.ID
		}
	}
	return "pane-chat"
}

func (u *UI) isConversationPane(id string) bool {
	p := layout.FindPane(u.hub.Store.Config().Layout, id)
	if p == nil {
		return false
	}
	ctx, ok := u.hub.Store.Context(p.ContextID)
	return ok && ctx.Kind == model.KindConversation
}

func (u *UI) resetLayout() {
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
		c.Layout = layout.Default("ctx-select", "ctx-chat", store.PanesContextID)
		c.SettingsRatio = 0.20
		c.SplitHandlePx = 4
	})
}

func (u *UI) mountInPane(paneID, contextID string) {
	cfg := u.hub.Store.Config()
	if p := layout.FindPane(cfg.Layout, paneID); p != nil {
		p.ContextID = contextID
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Layout = cfg.Layout })
	}
}

func (u *UI) onKey(ev *fyne.KeyEvent) {
	if u.recording != "" {
		if ev.Name == fyne.KeyEscape {
			u.recording = ""
			if u.inSettings {
				u.reopenSettings()
			}
			return
		}
		chord := formatChord(ev.Name, currentMods())
		u.saveChord(u.recording, chord)
		u.recording = ""
		if u.inSettings {
			u.openSettings()
		}
		return
	}
	if ev.Name == fyne.KeyEscape && u.inSettings {
		u.closeSettings()
	}
}

func (u *UI) saveChord(name, chord string) {
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
		if c.Keys == nil {
			c.Keys = store.DefaultKeys()
		}
		c.Keys[name] = chord
	})
}

func (u *UI) bindShortcuts() {
	add := func(name string, key fyne.KeyName, mod fyne.KeyModifier, fn func()) {
		u.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mod}, func(fyne.Shortcut) {
			if !store.BindingOn(u.hub.Store.Config(), name) {
				return
			}
			fn()
		})
	}
	add("settings", fyne.KeyComma, fyne.KeyModifierControl, u.openSettings)
	for _, ms := range markShortcuts {
		mark := ms.mark
		add(string(mark), ms.key, ms.mod, func() { u.applyMark(mark) })
	}
	add("next", fyne.KeyTab, fyne.KeyModifierControl, func() { u.cyclePane(1) })
	add("previous", fyne.KeyTab, fyne.KeyModifierControl|fyne.KeyModifierShift, func() { u.cyclePane(-1) })
}

// cyclePane moves the active pane to the next or previous chat pane.
func (u *UI) cyclePane(dir int) {
	panes := u.chatPanes()
	if len(panes) == 0 {
		return
	}
	idx := 0
	for i, p := range panes {
		if p.ID == u.focused {
			idx = i
		}
	}
	idx = (idx + dir + len(panes)) % len(panes)
	u.focusPane(panes[idx].ID)
	u.refresh()
	u.focusComposer(panes[idx].ID)
}

func modelID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (u *UI) authorLabel(from string) string {
	for _, id := range u.hub.Store.Identities() {
		if from == id.Public || from == id.ID || from == id.Label() || from == id.Name {
			return id.Label()
		}
	}
	for _, pin := range u.hub.Store.Pins() {
		if pin.Key == from || pin.Address == from {
			if pin.Name != "" {
				return pin.Name
			}
		}
	}
	if len(from) > 8 {
		return from[:8]
	}
	return from
}

func ellipsize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 28 {
		return s[:28] + "…"
	}
	return s
}
