package ui

import (
	"image/color"
	"net"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/layout"
	"lonchan/internal/model"
)

// SelectView lists users, groups, and chats.
type SelectView struct {
	widget.BaseWidget
	ui     *UI
	pane   *model.Node
	query  string
	filter *widget.Entry
	scope  *scopeBar
	rows   []selectRow
	index  int
}

type selectRow struct {
	title, sub, kind, key string
	account, accountID    string
	unread                int
	pending               bool
	ctx                   model.Context
	service               string
	targetKind            string
	target                string
}

func NewSelectView(ui *UI, pane *model.Node) *SelectView {
	v := &SelectView{ui: ui, pane: pane}
	v.ExtendBaseWidget(v)
	return v
}

func (v *SelectView) CreateRenderer() fyne.WidgetRenderer {
	root := container.NewMax()
	r := &selectRenderer{v: v, root: root}
	r.Refresh()
	return r
}

type selectRenderer struct {
	v    *SelectView
	root *fyne.Container
}

func (r *selectRenderer) Destroy() {}
func (r *selectRenderer) Layout(s fyne.Size) {
	r.root.Resize(s)
}
func (r *selectRenderer) MinSize() fyne.Size { return r.root.MinSize() }
func (r *selectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.root}
}
func (r *selectRenderer) Refresh() {
	r.root.Objects = []fyne.CanvasObject{keepScope(r.v, r.v.page())}
	r.root.Refresh()
}

func (v *SelectView) page() fyne.CanvasObject {
	v.rebuildRows()
	if v.filter == nil {
		v.filter = widget.NewEntry()
		v.filter.SetPlaceHolder("Filter")
		v.filter.ActionItem = widget.NewIcon(theme.SearchIcon())
		v.filter.OnChanged = func(s string) {
			v.query = s
			v.Refresh()
		}
	}
	if v.scope == nil {
		v.scope = newScopeBar(func(id string) {
			_ = v.ui.hub.Store.UpdateConfig(func(c *model.Config) { c.Select.Scope = id })
			v.Refresh()
		})
	}
	cfg := v.ui.hub.Store.Config()
	v.scope.setValue(cfg.Select.Scope)
	var body fyne.CanvasObject
	if scopeOf(cfg.Select.Scope) == scopeGroup {
		body = v.grouped(cfg)
	} else {
		body = v.flat(cfg)
	}
	ctx, ok := v.ui.hub.Store.Context(v.pane.ContextID)
	if !ok {
		ctx = model.Context{Name: "Selection", Kind: model.KindSelect}
	}
	head := []fyne.CanvasObject{v.filter, v.scope}
	if v.pane.ShowHeader() {
		head = append([]fyne.CanvasObject{v.ui.paneHeader(v.pane, ctx)}, head...)
	}
	return container.NewBorder(container.NewVBox(head...), nil, nil, nil, container.NewVScroll(body))
}

func (v *SelectView) activeContextID(cfg model.Config) string {
	if p := layout.FindPane(cfg.Layout, v.ui.focused); p != nil && v.ui.isConversationPane(p.ID) {
		return p.ContextID
	}
	return ""
}

func (v *SelectView) flat(cfg model.Config) fyne.CanvasObject {
	box := container.NewVBox()
	scope := scopeOf(cfg.Select.Scope)
	focused := v.ui.focusedConversation().IdentityID
	active := v.activeContextID(cfg)
	add := func(title string, open bool, kind string, toggle func(bool)) {
		box.Add(v.section(title, open, kind, "", scope, focused, active, true, toggle))
	}
	add("Users", cfg.Select.UsersOpen, "user", func(o bool) {
		_ = v.ui.hub.Store.UpdateConfig(func(c *model.Config) { c.Select.UsersOpen = o })
		v.Refresh()
	})
	add("Groups", cfg.Select.GroupsOpen, "group", func(o bool) {
		_ = v.ui.hub.Store.UpdateConfig(func(c *model.Config) { c.Select.GroupsOpen = o })
		v.Refresh()
	})
	add("Chats", cfg.Select.ChatsOpen, "chat", func(o bool) {
		_ = v.ui.hub.Store.UpdateConfig(func(c *model.Config) { c.Select.ChatsOpen = o })
		v.Refresh()
	})
	return box
}

func (v *SelectView) grouped(cfg model.Config) fyne.CanvasObject {
	box := container.NewVBox()
	active := v.activeContextID(cfg)
	rows := container.NewVBox()
	if v.fill(rows, "user", "", scopeAll, "", active, true) > 0 {
		box.Add(container.NewVBox(sectionHead("Pending", true, nil), rows))
	}
	for _, a := range v.accounts() {
		if !v.accountShown(a.id) {
			continue
		}
		id := a.id
		open := groupShown(cfg, id)
		head := sectionHead(a.label, open, func() {
			_ = v.ui.hub.Store.UpdateConfig(func(c *model.Config) {
				if c.Select.GroupOpen == nil {
					c.Select.GroupOpen = map[string]bool{}
				}
				c.Select.GroupOpen[id] = !open
			})
			v.Refresh()
		})
		section := container.NewVBox(head)
		if open {
			cats := container.NewVBox()
			for _, cat := range []struct {
				title, kind string
				open        bool
			}{
				{"Users", "user", cfg.Select.UsersOpen},
				{"Groups", "group", cfg.Select.GroupsOpen},
				{"Chats", "chat", cfg.Select.ChatsOpen},
			} {
				block := v.section(cat.title, cat.open, cat.kind, id, scopeAll, "", active, false, func(o bool) {
					_ = v.ui.hub.Store.UpdateConfig(func(c *model.Config) {
						switch cat.kind {
						case "user":
							c.Select.UsersOpen = o
						case "group":
							c.Select.GroupsOpen = o
						default:
							c.Select.ChatsOpen = o
						}
					})
					v.Refresh()
				})
				if block != nil {
					cats.Add(indent(block))
				}
			}
			section.Add(cats)
		}
		box.Add(section)
	}
	if len(box.Objects) == 0 {
		box.Add(mutedLabel("None"))
	}
	return box
}

func sectionHead(title string, open bool, toggle func()) *widget.Button {
	icon := theme.MenuExpandIcon()
	if open {
		icon = theme.MenuDropDownIcon()
	}
	head := widget.NewButtonWithIcon(title, icon, func() {
		if toggle != nil {
			toggle()
		}
	})
	head.Importance = widget.LowImportance
	head.Alignment = widget.ButtonAlignLeading
	return head
}

func indent(o fyne.CanvasObject) fyne.CanvasObject {
	pad := canvas.NewRectangle(color.Transparent)
	pad.SetMinSize(fyne.NewSize(theme.Padding()*3, 1))
	return container.NewBorder(nil, nil, pad, nil, o)
}

// section is one Users, Groups, or Chats block. accountID limits the rows to that account.
// An empty accountID lists every account. omitEmpty drops the block when it has no rows.
func (v *SelectView) section(title string, open bool, kind, accountID, scope, focusedID, active string, showEmpty bool, toggle func(bool)) fyne.CanvasObject {
	rows := container.NewVBox()
	n := v.fill(rows, kind, accountID, scope, focusedID, active, false)
	if n == 0 && !showEmpty {
		return nil
	}
	head := sectionHead(title, open, func() { toggle(!open) })
	add := iconButton(theme.ContentAddIcon(), func() { v.ui.addTarget(kind) })
	section := container.NewVBox(container.NewBorder(nil, nil, nil, add, head))
	if open {
		if n == 0 {
			section.Add(mutedLabel("None"))
		} else {
			section.Add(rows)
		}
	}
	return section
}

func (v *SelectView) fill(section *fyne.Container, kind, accountID, scope, focusedID, active string, pendingOnly bool) int {
	shown := 0
	for i, row := range v.rows {
		if !rowListed(row, kind, accountID, scope, focusedID, v.query, pendingOnly) {
			continue
		}
		idx := i
		line := row
		shown++
		here := line.ctx.ID != "" && line.ctx.ID == active
		cl := newChatLine(line.title, line.sub, line.unread, here, line.pending, func() {
			v.index = idx
			if line.pending {
				ctx, err := v.ui.hub.Accept(line.key)
				if err == nil {
					v.ui.mountConversation(ctx.ID)
				}
				return
			}
			v.mount(line)
		})
		if line.ctx.ID != "" {
			cl.menu = func() []*fyne.MenuItem { return v.ui.openChatItems(line.ctx.ID) }
		}
		section.Add(cl)
	}
	return shown
}

func (v *SelectView) accounts() []struct{ id, label string } {
	seen := map[string]string{}
	var ids []string
	for _, row := range v.rows {
		if row.pending || row.kind != "chat" {
			continue
		}
		if _, ok := seen[row.accountID]; ok {
			continue
		}
		seen[row.accountID] = row.account
		ids = append(ids, row.accountID)
	}
	sort.Slice(ids, func(i, j int) bool {
		return strings.ToLower(seen[ids[i]]) < strings.ToLower(seen[ids[j]])
	})
	out := make([]struct{ id, label string }, len(ids))
	for i, id := range ids {
		out[i] = struct{ id, label string }{id, seen[id]}
	}
	return out
}

func (v *SelectView) accountShown(id string) bool {
	for _, row := range v.rows {
		if row.pending || row.ctx.IdentityID != id {
			continue
		}
		if v.query != "" && !strings.Contains(strings.ToLower(row.title), strings.ToLower(v.query)) {
			continue
		}
		return true
	}
	return false
}

func groupShown(cfg model.Config, id string) bool {
	if cfg.Select.GroupOpen == nil {
		return true
	}
	open, ok := cfg.Select.GroupOpen[id]
	if !ok {
		return true
	}
	return open
}

// rowListed reports whether a row belongs in a section.
// accountID empty means every account. pendingOnly keeps inbound peers and nothing else.
func rowListed(row selectRow, kind, accountID, scope, focusedID, query string, pendingOnly bool) bool {
	if row.kind != kind {
		return false
	}
	if pendingOnly {
		if !row.pending {
			return false
		}
	} else if row.pending && accountID != "" {
		return false
	} else if accountID != "" && row.ctx.IdentityID != accountID {
		return false
	}
	if !rowKept(scope, focusedID, row.ctx.IdentityID, row.pending) {
		return false
	}
	if query != "" && !strings.Contains(strings.ToLower(row.title), strings.ToLower(query)) {
		return false
	}
	return true
}

// rowKept applies the Mine scope. Any other scope keeps the row.
// With no focused account, Mine shows every row.
func rowKept(scope, focusedID, rowIdentity string, pending bool) bool {
	if scopeOf(scope) != scopeMine || focusedID == "" {
		return true
	}
	if pending {
		return false
	}
	return rowIdentity == focusedID
}

// panesShowing names the panes showing a context: "Pane 2", "Pane 2, Pane 4".
func (u *UI) panesShowing(contextID string) string {
	var parts []string
	for _, p := range layout.Panes(u.hub.Store.Config().Layout) {
		if p.ContextID == contextID {
			parts = append(parts, "Pane "+itoa(u.paneNumber(p.ID)))
		}
	}
	return strings.Join(parts, ", ")
}

func (v *SelectView) rebuildRows() {
	v.rows = nil
	for _, p := range v.ui.hub.Pending() {
		label := p.Key
		if len(label) > 8 {
			label = label[:8]
		}
		v.rows = append(v.rows, selectRow{title: label, sub: p.Key, kind: "user", key: p.Key, pending: true})
	}
	focused := v.ui.focusedConversation()
	for _, c := range v.ui.hub.Store.Contexts() {
		// A conversation with no chat in it is an empty pane's placeholder, not a chat.
		if c.Kind != model.KindConversation || c.Target == "" {
			continue
		}
		svc, _ := v.ui.hub.Store.Service(c.ServiceID)
		msgs, _ := v.ui.hub.Store.ReadLog(c.ServiceID, c.Target)
		preview := ""
		unread := 0
		seen := c.LastReadID == ""
		for _, m := range msgs {
			if !seen {
				if m.ID == c.LastReadID {
					seen = true
				}
				continue
			}
			if c.LastReadID != "" && m.ID != c.LastReadID {
				unread++
			}
			preview = Plain(m.Body)
		}
		if c.LastReadID == "" {
			unread = len(msgs)
		}
		if focused.ID == c.ID {
			unread = 0
		}
		identLabel := c.IdentityID
		if id, ok := v.ui.hub.Store.Identity(c.IdentityID); ok {
			identLabel = id.Label()
		}
		// Where the chat is open comes first, so it is visible even when truncated.
		where := v.ui.panesShowing(c.ID)
		join := func(parts ...string) string {
			var out []string
			for _, p := range parts {
				if p != "" {
					out = append(out, p)
				}
			}
			return strings.Join(out, " · ")
		}
		title := c.Name
		if title == "" {
			title = c.Target
		}
		// A context here is the connection (IRC, direct, and so on). The account is who you are on it.
		account := identLabel
		if account == "" {
			account = "No account"
		}
		attached := join(account, connectionLabel(svc))
		v.rows = append(v.rows, selectRow{title: title, sub: join(attached, where, preview), unread: unread, kind: "chat", account: account, accountID: c.IdentityID, ctx: c})
		if c.TargetKind == model.TargetUser {
			v.rows = append(v.rows, selectRow{title: title, sub: attached, kind: "user", account: account, accountID: c.IdentityID, ctx: c, service: c.ServiceID, targetKind: c.TargetKind, target: c.Target})
		}
		if c.TargetKind == model.TargetGroup {
			members := itoa(len(v.ui.hub.Members(c.Target))) + " members"
			v.rows = append(v.rows, selectRow{title: title, sub: join(attached, members), kind: "group", account: account, accountID: c.IdentityID, ctx: c, service: c.ServiceID, targetKind: c.TargetKind, target: c.Target})
		}
	}
}

// connectionLabel is the context a person is attached to: the kind of connection and where it goes.
func connectionLabel(svc model.Service) string {
	cfg := svc.Config
	if cfg == nil {
		cfg = map[string]string{}
	}
	switch svc.Plugin {
	case model.PluginIRC:
		if cfg["host"] != "" {
			return "IRC · " + cfg["host"]
		}
		return "IRC"
	case model.PluginXMPP:
		if cfg["jid"] != "" {
			return "XMPP · " + cfg["jid"]
		}
		if cfg["host"] != "" {
			return "XMPP · " + cfg["host"]
		}
		return "XMPP"
	case model.PluginDirect:
		return "Direct"
	case model.PluginLocal:
		return "Local"
	default:
		if svc.Plugin != "" {
			return svc.Plugin
		}
		return svc.ID
	}
}

func (v *SelectView) mount(row selectRow) {
	// Users and Groups rows come from a conversation; open that same conversation.
	if row.ctx.ID != "" {
		v.ui.openChat(row.ctx.ID)
		return
	}
	ident := ""
	if ids := v.ui.hub.Store.Identities(); len(ids) > 0 {
		ident = ids[0].ID
	}
	svc := row.service
	if svc == "" {
		svc = "svc-local"
	}
	ctx, err := v.ui.hub.EnsureConversation(svc, ident, row.targetKind, row.target, row.title)
	if err != nil {
		return
	}
	v.ui.mountConversation(ctx.ID)
}

func (u *UI) addTarget(kind string) {
	var opts []string
	var svcs []model.Service
	for _, s := range u.hub.Store.Services() {
		if kind == "group" && s.Plugin == model.PluginDirect && s.Config["relay"] != "true" {
			continue
		}
		opts = append(opts, s.ID)
		svcs = append(svcs, s)
	}
	if len(opts) == 0 {
		dialog.ShowInformation("Add "+kind, "No matching service.", u.win)
		return
	}
	sel := widget.NewSelect(opts, nil)
	sel.SetSelected(opts[0])
	d := dialog.NewForm("Add "+kind, "Next", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Service", sel),
	}, func(ok bool) {
		if !ok {
			return
		}
		var svc model.Service
		for _, s := range svcs {
			if s.ID == sel.Selected {
				svc = s
			}
		}
		u.addTargetFields(kind, svc)
	}, u.win)
	d.Show()
}

func (u *UI) addTargetFields(kind string, svc model.Service) {
	targetKind := model.TargetUser
	if kind == "group" {
		targetKind = model.TargetGroup
	}
	finish := func(target, name string) {
		if target == "" {
			return
		}
		ctx, err := u.hub.EnsureConversation(svc.ID, svc.IdentityID, targetKind, target, name)
		if err == nil {
			u.mountConversation(ctx.ID)
		}
	}
	switch {
	case kind == "user" && svc.Plugin == model.PluginLocal:
		var ids []string
		var labels []string
		for _, id := range u.hub.Store.Identities() {
			if id.ID == svc.IdentityID {
				continue
			}
			ids = append(ids, id.ID)
			labels = append(labels, id.Label())
		}
		if len(ids) == 0 {
			dialog.ShowInformation("Add user", "No other identity.", u.win)
			return
		}
		sel := widget.NewSelect(labels, nil)
		sel.SetSelected(labels[0])
		dialog.NewForm("Local user", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Identity", sel),
		}, func(ok bool) {
			if !ok {
				return
			}
			for i, label := range labels {
				if label == sel.Selected {
					finish(ids[i], label)
				}
			}
		}, u.win).Show()
	case kind == "user" && svc.Plugin == model.PluginDirect:
		host := widget.NewEntry()
		host.SetText("127.0.0.1")
		port := widget.NewEntry()
		pin := widget.NewEntry()
		dialog.NewForm("Direct user", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Host", host),
			widget.NewFormItem("Port", port),
			widget.NewFormItem("Pin", pin),
		}, func(ok bool) {
			if !ok || port.Text == "" {
				return
			}
			addr := net.JoinHostPort(host.Text, port.Text)
			if pin.Text != "" {
				_ = u.hub.Store.PutPin(model.Pin{Address: addr, Key: pin.Text})
			}
			finish(addr, addr)
		}, u.win).Show()
	case kind == "user" && svc.Plugin == model.PluginIRC:
		nick := widget.NewEntry()
		dialog.NewForm("IRC user", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Nick", nick),
		}, func(ok bool) {
			if ok {
				finish(nick.Text, nick.Text)
			}
		}, u.win).Show()
	case kind == "user" && svc.Plugin == model.PluginXMPP:
		jid := widget.NewEntry()
		dialog.NewForm("XMPP user", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("JID", jid),
		}, func(ok bool) {
			if ok {
				finish(jid.Text, jid.Text)
			}
		}, u.win).Show()
	case kind == "group" && svc.Plugin == model.PluginLocal:
		room := widget.NewEntry()
		dialog.NewForm("Local group", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Room", room),
		}, func(ok bool) {
			if ok {
				finish(room.Text, room.Text)
			}
		}, u.win).Show()
	case kind == "group" && svc.Plugin == model.PluginDirect:
		room := widget.NewEntry()
		dialog.NewForm("Direct group", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Room", room),
			widget.NewFormItem("Listener", widget.NewLabel(svc.ID)),
		}, func(ok bool) {
			if ok {
				finish(room.Text, room.Text)
			}
		}, u.win).Show()
	case kind == "group" && svc.Plugin == model.PluginIRC:
		channel := widget.NewEntry()
		channel.SetText("#")
		dialog.NewForm("IRC channel", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Channel", channel),
		}, func(ok bool) {
			if !ok {
				return
			}
			name := channel.Text
			if name != "" && !strings.HasPrefix(name, "#") {
				name = "#" + name
			}
			finish(name, name)
		}, u.win).Show()
	case kind == "group" && svc.Plugin == model.PluginXMPP:
		room := widget.NewEntry()
		dialog.NewForm("XMPP room", "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Room JID", room),
		}, func(ok bool) {
			if ok {
				finish(room.Text, room.Text)
			}
		}, u.win).Show()
	default:
		target := widget.NewEntry()
		dialog.NewForm("Add "+kind, "Add", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Target", target),
		}, func(ok bool) {
			if ok {
				finish(target.Text, target.Text)
			}
		}, u.win).Show()
	}
}

func (v *SelectView) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyUp:
		if v.index > 0 {
			v.index--
		}
	case fyne.KeyDown:
		if v.index < len(v.rows)-1 {
			v.index++
		}
	case fyne.KeyReturn, fyne.KeyEnter:
		if v.index >= 0 && v.index < len(v.rows) {
			v.mount(v.rows[v.index])
		}
	}
	v.Refresh()
}

func (v *SelectView) FocusGained()   {}
func (v *SelectView) FocusLost()     {}
func (v *SelectView) TypedRune(rune) {}

type chatLine struct {
	widget.BaseWidget
	title, sub        string
	unread            int
	selected, pending bool
	hovered           bool
	tap               func()
	menu              func() []*fyne.MenuItem // right-click choices, or nil
}

func newChatLine(title, sub string, unread int, selected, pending bool, tap func()) *chatLine {
	c := &chatLine{title: title, sub: sub, unread: unread, selected: selected, pending: pending, tap: tap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *chatLine) Tapped(*fyne.PointEvent) {
	if c.tap != nil {
		c.tap()
	}
}

// TappedSecondary shows the row's right-click menu.
func (c *chatLine) TappedSecondary(ev *fyne.PointEvent) {
	if c.menu == nil {
		return
	}
	if items := c.menu(); len(items) > 0 {
		if cv := fyne.CurrentApp().Driver().CanvasForObject(c); cv != nil {
			widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", items...), cv, ev.AbsolutePosition)
		}
	}
}

func (c *chatLine) MouseIn(*desktop.MouseEvent) {
	c.hovered = true
	c.Refresh()
}
func (c *chatLine) MouseMoved(*desktop.MouseEvent) {}
func (c *chatLine) MouseOut() {
	c.hovered = false
	c.Refresh()
}
func (c *chatLine) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (c *chatLine) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.InputRadiusSize()
	title := canvas.NewText(c.title, theme.ColorForWidget(theme.ColorNameForeground, c))
	if c.unread > 0 {
		title.TextStyle.Bold = true
	}
	sub := canvas.NewText(c.sub, theme.ColorForWidget(theme.ColorNamePlaceHolder, c))
	sub.TextSize = theme.SizeForWidget(theme.SizeNameCaptionText, c)
	r := &chatLineRenderer{line: c, bg: bg, title: title, sub: sub}
	if c.unread > 0 {
		r.badge = canvas.NewRectangle(theme.ColorForWidget(theme.ColorNamePrimary, c))
		r.count = canvas.NewText(itoa(c.unread), theme.ColorForWidget(theme.ColorNameForegroundOnPrimary, c))
		r.count.TextSize = theme.SizeForWidget(theme.SizeNameCaptionText, c)
		r.count.TextStyle.Bold = true
		r.count.Alignment = fyne.TextAlignCenter
	}
	if c.pending {
		r.accept = widget.NewButton("Accept", func() {
			if c.tap != nil {
				c.tap()
			}
		})
		r.accept.Importance = widget.HighImportance
	}
	r.Refresh()
	return r
}

type chatLineRenderer struct {
	line   *chatLine
	bg     *canvas.Rectangle
	title  *canvas.Text
	sub    *canvas.Text
	badge  *canvas.Rectangle
	count  *canvas.Text
	accept *widget.Button
}

func (r *chatLineRenderer) Destroy() {}
func (r *chatLineRenderer) Objects() []fyne.CanvasObject {
	out := []fyne.CanvasObject{r.bg, r.title, r.sub}
	if r.badge != nil {
		out = append(out, r.badge, r.count)
	}
	if r.accept != nil {
		out = append(out, r.accept)
	}
	return out
}

const chatLinePad = 8

func (r *chatLineRenderer) MinSize() fyne.Size {
	h := fyne.MeasureText("Ag", r.title.TextSize, r.title.TextStyle).Height +
		fyne.MeasureText("Ag", r.sub.TextSize, r.sub.TextStyle).Height + chatLinePad + 2
	if r.accept != nil {
		if ah := r.accept.MinSize().Height + 4; ah > h {
			h = ah
		}
	}
	// Width is not reserved for the text: it truncates, so narrow panes never scroll sideways.
	return fyne.NewSize(48, h)
}

func (r *chatLineRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	right := s.Width - chatLinePad
	if r.accept != nil {
		aw := r.accept.MinSize()
		r.accept.Resize(aw)
		r.accept.Move(fyne.NewPos(right-aw.Width, (s.Height-aw.Height)/2))
		right -= aw.Width + theme.Padding()
	}
	if r.badge != nil {
		cs := fyne.MeasureText(r.count.Text, r.count.TextSize, r.count.TextStyle)
		bh := cs.Height + 2
		bw := cs.Width + 10
		if bw < bh {
			bw = bh
		}
		r.badge.CornerRadius = bh / 2
		r.badge.Resize(fyne.NewSize(bw, bh))
		r.badge.Move(fyne.NewPos(right-bw, (s.Height-bh)/2))
		r.count.Resize(fyne.NewSize(bw, bh))
		r.count.Move(r.badge.Position())
		right -= bw + theme.Padding()
	}
	textW := right - chatLinePad
	r.title.Text = fitText(r.line.title, textW, r.title.TextSize, r.title.TextStyle)
	r.sub.Text = fitText(r.line.sub, textW, r.sub.TextSize, r.sub.TextStyle)
	th := r.title.MinSize().Height
	sh := r.sub.MinSize().Height
	top := (s.Height - th - sh) / 2
	r.title.Move(fyne.NewPos(chatLinePad, top))
	r.title.Resize(fyne.NewSize(textW, th))
	r.sub.Move(fyne.NewPos(chatLinePad, top+th))
	r.sub.Resize(fyne.NewSize(textW, sh))
	r.title.Refresh()
	r.sub.Refresh()
}

func (r *chatLineRenderer) Refresh() {
	switch {
	case r.line.selected:
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNameSelection, r.line)
	case r.line.hovered:
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNameHover, r.line)
	default:
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
}

func (v *SelectView) MouseIn(*desktop.MouseEvent)    {}
func (v *SelectView) MouseMoved(*desktop.MouseEvent) {}
func (v *SelectView) MouseOut()                      {}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
