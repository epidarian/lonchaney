package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/caps"
	"lonchan/internal/model"
)

// InspectorView shows the focused conversation's identity, service, marks, and members.
func InspectorView(ui *UI, ctx model.Context) fyne.CanvasObject {
	focused := ui.focusedConversation()
	if focused.ID != "" {
		ctx = focused
	}
	box := container.NewVBox()
	field := func(name string, value fyne.CanvasObject) {
		box.Add(inspectorKey(name))
		box.Add(value)
	}
	if id, ok := ui.hub.Store.Identity(ctx.IdentityID); ok {
		who := container.NewHBox(newAvatar(id.Label(), ui.hub.Store.AbsIcon(id.Icon), nil), truncLabel(id.Label()))
		field("Identity", who)
		if id.Username != "" {
			field("Username", truncLabel(id.Username))
		}
		if id.Pronouns != "" {
			field("Pronouns", truncLabel(id.Pronouns))
		}
		fp := monoLabel(id.Public)
		fp.Importance = widget.LowImportance
		field("Fingerprint", fp)
	}
	svc, ok := ui.hub.Store.Service(ctx.ServiceID)
	pluginName := ""
	if ok {
		pluginName = svc.Plugin
		field("Service", truncLabel(svc.ID+" · "+svc.Plugin))
		status := ui.hub.Status(svc.ID)
		if status == "" {
			status = svc.Status
		}
		if status == "" {
			status = "—"
		}
		field("Status", truncLabel(status))
	}
	c := ui.hub.Caps(pluginName)
	marks := container.NewVBox()
	for _, m := range caps.All() {
		name := theme.ColorNameForeground
		if !c.Has(m) || caps.HeaderGrey(c, ui.hovered) && ui.hovered == m {
			name = theme.ColorNameDisabled
		}
		marks.Add(canvasLabel(string(m), name))
	}
	field("Marks", marks)
	if ctx.TargetKind == model.TargetGroup {
		members := container.NewVBox()
		for _, member := range ui.groupMembers(ctx) {
			members.Add(truncLabel(member))
		}
		field("Members", members)
	}
	if pluginName == model.PluginDirect {
		notes := container.NewVBox()
		if pin, ok := ui.hub.Store.Pin(ctx.Target); ok {
			field("Pinned key", monoLabel(pin.Key))
		}
		if id, ok := ui.hub.Store.Identity(ctx.IdentityID); ok {
			if note := ui.hub.Store.Config().Handshake[id.ID]; note != "" {
				notes.Add(hint(note))
			}
		}
		if svc.Config["relay"] == "true" {
			notes.Add(hint("Host can read relayed messages."))
		}
		if len(notes.Objects) > 0 {
			field("Notes", notes)
		}
	}
	if len(box.Objects) == 0 {
		box.Add(hint("Nothing to inspect yet."))
	}
	return container.NewVScroll(box)
}

// inspectorKey is the small muted heading above an inspector value.
func inspectorKey(s string) fyne.CanvasObject {
	l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.Importance = widget.LowImportance
	l.SizeName = theme.SizeNameCaptionText
	return container.New(&tightTop{}, l)
}

// tightTop trims the label's bottom padding so a key sits close to its value.
type tightTop struct{}

func (tightTop) MinSize(objs []fyne.CanvasObject) fyne.Size {
	m := objs[0].MinSize()
	return fyne.NewSize(m.Width, m.Height-theme.InnerPadding())
}

func (tightTop) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Resize(fyne.NewSize(size.Width, size.Height+theme.InnerPadding()))
	objs[0].Move(fyne.NewPos(0, theme.InnerPadding()/2))
}

func (u *UI) groupMembers(ctx model.Context) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, c := range u.hub.Store.Contexts() {
		if c.Kind == model.KindConversation && c.ServiceID == ctx.ServiceID && c.Target == ctx.Target {
			if id, ok := u.hub.Store.Identity(c.IdentityID); ok {
				add(id.Label())
			}
		}
	}
	for _, m := range u.hub.Members(ctx.ServiceID) {
		add(m)
	}
	for _, m := range u.hub.Members(ctx.Target) {
		add(m)
	}
	return out
}

// canvasLabel is a compact one-line text in the named color, inset like a Label.
func canvasLabel(text string, name fyne.ThemeColorName) fyne.CanvasObject {
	return newTintText(text, name)
}
