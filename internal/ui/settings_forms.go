package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/layout"
	"lonchan/internal/model"
	"lonchan/internal/plugin"
	"lonchan/internal/store"
)

func (u *UI) contextSettings() fyne.CanvasObject {
	var conns []model.Context
	for _, c := range u.hub.Store.Contexts() {
		if c.Plugin != "" && c.ServiceID != "" {
			conns = append(conns, c)
		}
	}
	found := false
	for _, c := range conns {
		if c.ID == u.contextPick {
			found = true
		}
	}
	if !found && len(conns) > 0 {
		u.contextPick = conns[0].ID
	}
	var labels, ids []string
	for _, c := range conns {
		label := c.Name
		if label == "" {
			label = c.Plugin
		}
		labels = append(labels, label+" ("+c.ID+")")
		ids = append(ids, c.ID)
	}
	picker := widget.NewSelect(labels, nil)
	for i, id := range ids {
		if id == u.contextPick {
			picker.SetSelected(labels[i])
		}
	}
	picker.OnChanged = func(label string) {
		for i, l := range labels {
			if l == label {
				u.contextPick = ids[i]
			}
		}
		u.reopenSettings()
	}
	add := widget.NewButtonWithIcon("Add connection", theme.ContentAddIcon(), func() {
		ident := ""
		if list := u.hub.Store.Identities(); len(list) > 0 {
			ident = list[0].ID
		}
		svcID := "svc-" + modelID()
		ctxID := "ctx-" + modelID()
		svc := model.Service{ID: svcID, Plugin: model.PluginIRC, IdentityID: ident, Config: map[string]string{}}
		ctx := model.Context{
			ID: ctxID, Name: "IRC", Kind: model.KindConversation,
			ServiceID: svcID, IdentityID: ident, Plugin: model.PluginIRC, Config: map[string]string{},
		}
		_ = u.hub.Store.PutService(svc)
		_ = u.hub.Store.PutContext(ctx)
		u.contextPick = ctxID
		u.reopenSettings()
	})
	out := container.NewVBox(form(formLabel("Context"), container.NewBorder(nil, nil, nil, add, picker)))

	for _, ctx := range conns {
		c := ctx
		if c.ID != u.contextPick {
			continue
		}
		if c.Config == nil {
			c.Config = map[string]string{}
		}
		name := widget.NewEntry()
		name.SetText(c.Name)
		name.OnChanged = func(s string) {
			c.Name = s
			u.writeConnection(c)
		}
		plugins := u.pluginIDs()
		plug := widget.NewSelect(plugins, nil)
		plug.SetSelected(c.Plugin)
		plug.OnChanged = func(p string) {
			if p == c.Plugin {
				return
			}
			c.Plugin = p
			u.writeConnection(c)
			u.reopenSettings()
		}
		idents := u.identityChoices()
		ident := widget.NewSelect(idents.labels, nil)
		ident.SetSelected(idents.label(c.IdentityID))
		ident.OnChanged = func(label string) {
			c.IdentityID = idents.id(label)
			u.writeConnection(c)
		}
		details := form(
			formLabel("Name"), name,
			formLabel("Account"), width(220, ident),
			formLabel("Plugin"), width(220, plug),
		)
		u.addConnectionFields(details, c.Plugin, c.Config, func() { u.writeConnection(c) })
		svc, _ := u.hub.Store.Service(c.ServiceID)
		status := u.hub.Status(c.ServiceID)
		if status == "" {
			status = "Not connected"
		}
		conn := widget.NewButtonWithIcon("Connect", theme.MediaPlayIcon(), func() {
			if err := u.hub.Connect(svc); err != nil {
				dialog.ShowError(err, u.win)
			}
			u.reopenSettings()
		})
		conn.Importance = widget.HighImportance
		disc := widget.NewButtonWithIcon("Disconnect", theme.MediaStopIcon(), func() {
			u.hub.Disconnect(svc)
			u.reopenSettings()
		})
		people := container.NewVBox()
		for _, other := range u.hub.Store.Contexts() {
			if other.ServiceID != c.ServiceID || other.Target == "" {
				continue
			}
			who := other.Name
			if who == "" {
				who = other.Target
			}
			account := c.IdentityID
			if id, ok := u.hub.Store.Identity(other.IdentityID); ok {
				account = id.Label()
			}
			people.Add(widget.NewLabel(who + " · " + account + " · " + connectionLabel(svc) + " · " + other.Target))
		}
		if len(people.Objects) == 0 {
			people.Add(hint("No people on this connection yet."))
		}
		personName := widget.NewEntry()
		personName.SetPlaceHolder("Name")
		personAddr := widget.NewEntry()
		personAddr.SetPlaceHolder("nick, JID, room, or host:port")
		kind := widget.NewSelect([]string{model.TargetUser, model.TargetGroup}, nil)
		kind.SetSelected(model.TargetUser)
		addPerson := widget.NewButtonWithIcon("Add person", theme.ContentAddIcon(), func() {
			if personAddr.Text == "" {
				return
			}
			_ = u.hub.Store.PutContext(model.Context{
				ID: modelID(), Name: personName.Text, Kind: model.KindConversation,
				ServiceID: c.ServiceID, IdentityID: c.IdentityID,
				TargetKind: kind.Selected, Target: personAddr.Text,
			})
			u.reopenSettings()
		})
		del := widget.NewButtonWithIcon("Delete context", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Delete context", "Delete "+c.Name+" and its connection?", func(ok bool) {
				if !ok {
					return
				}
				_ = u.hub.Store.DeleteService(c.ServiceID)
				u.removeContext(c.ID)
				u.contextPick = ""
				u.reopenSettings()
			}, u.win)
		})
		del.Importance = widget.DangerImportance
		out.Add(group("Connection", details, hint(status), trailing(disc, conn)))
		out.Add(group("People", people, container.NewBorder(nil, nil, nil, addPerson, container.NewHBox(width(160, personName), width(220, personAddr), width(120, kind)))))
		out.Add(container.NewHBox(del))
	}
	return out
}

func (u *UI) pluginIDs() []string {
	ids := []string{model.PluginLocal, model.PluginDirect, model.PluginIRC, model.PluginXMPP}
	for _, p := range u.hub.Store.Config().Plugins {
		if p.Enabled && p.ID != "" {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func (u *UI) connectionSpec(id string) (string, []model.Field) {
	if text, fields, ok := plugin.Builtin(id); ok {
		return text, fields
	}
	for _, p := range u.hub.Store.Config().Plugins {
		if p.ID == id {
			return p.Instructions, p.Fields
		}
	}
	return "", nil
}

func (u *UI) syncContextFromService(svc model.Service) {
	for _, c := range u.hub.Store.Contexts() {
		if c.Plugin == "" || c.ServiceID != svc.ID {
			continue
		}
		c.Plugin = svc.Plugin
		c.IdentityID = svc.IdentityID
		c.Config = svc.Config
		_ = u.hub.Store.PutContext(c)
	}
}

func (u *UI) writeConnection(c model.Context) {
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	_ = u.hub.Store.PutContext(c)
	svc, ok := u.hub.Store.Service(c.ServiceID)
	if !ok {
		svc = model.Service{ID: c.ServiceID, Config: map[string]string{}}
	}
	svc.Plugin = c.Plugin
	svc.IdentityID = c.IdentityID
	svc.Config = c.Config
	_ = u.hub.Store.PutService(svc)
}

func (u *UI) addConnectionFields(fields *fyne.Container, pluginID string, cfg map[string]string, save func()) {
	text, spec := u.connectionSpec(pluginID)
	if text != "" {
		fields.Add(widget.NewLabel(""))
		fields.Add(hint(text))
	}
	if cfg == nil {
		return
	}
	for _, f := range spec {
		f := f
		if f.Bool {
			check := widget.NewCheck(f.Label, nil)
			check.SetChecked(cfg[f.Key] == "true")
			check.OnChanged = func(on bool) {
				if on {
					cfg[f.Key] = "true"
				} else {
					cfg[f.Key] = ""
				}
				save()
			}
			fields.Add(widget.NewLabel(""))
			fields.Add(check)
			continue
		}
		e := widget.NewEntry()
		if f.Secret {
			e = widget.NewPasswordEntry()
		}
		e.SetPlaceHolder(f.Placeholder)
		e.SetText(cfg[f.Key])
		e.OnChanged = func(v string) {
			cfg[f.Key] = v
			save()
		}
		label := f.Label
		if f.Help != "" {
			label = f.Label + " — " + f.Help
		}
		fields.Add(formLabel(label))
		fields.Add(e)
	}
}

func serviceTitle(s model.Service) string {
	detail := s.ID
	if s.Config != nil {
		for _, key := range []string{"jid", "nick", "host"} {
			if s.Config[key] != "" {
				detail = s.Config[key]
				break
			}
		}
	}
	return s.Plugin + " · " + detail
}

func (u *UI) identitySettings() fyne.CanvasObject {
	ids := u.hub.Store.Identities()
	found := false
	for _, id := range ids {
		if id.ID == u.identityPick {
			found = true
		}
	}
	if !found && len(ids) > 0 {
		u.identityPick = ids[0].ID
	}
	all := u.identityChoices()
	picker := widget.NewSelect(all.labels, nil)
	picker.SetSelected(all.label(u.identityPick))
	picker.OnChanged = func(label string) {
		u.identityPick = all.id(label)
		u.reopenSettings()
	}
	create := widget.NewButtonWithIcon("New identity", theme.ContentAddIcon(), func() {
		id, err := u.hub.Store.AddIdentity("")
		if err == nil {
			u.identityPick = id.ID
		}
		u.reopenSettings()
	})
	out := container.NewVBox(form(formLabel("Identity"), container.NewBorder(nil, nil, nil, create, picker)))
	for _, id := range ids {
		ident := id
		if ident.ID != u.identityPick {
			continue
		}
		save := func() { _ = u.hub.Store.PutIdentity(ident) }
		name := widget.NewEntry()
		name.SetPlaceHolder("Display name")
		name.SetText(ident.Name)
		name.OnChanged = func(s string) {
			ident.Name = s
			save()
		}
		user := widget.NewEntry()
		user.SetPlaceHolder("username")
		user.SetText(ident.Username)
		user.OnChanged = func(s string) {
			ident.Username = s
			save()
		}
		pronouns := widget.NewEntry()
		pronouns.SetPlaceHolder("they/them")
		pronouns.SetText(ident.Pronouns)
		pronouns.OnChanged = func(s string) {
			ident.Pronouns = s
			save()
		}
		face := newAvatarSize(ident.Label(), u.hub.Store.AbsIcon(ident.Icon), 64, nil)
		choose := widget.NewButton("Choose image", func() {
			d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil {
					dialog.ShowError(err, u.win)
					return
				}
				if r == nil {
					return
				}
				defer r.Close()
				data, err := io.ReadAll(r)
				if err != nil {
					dialog.ShowError(err, u.win)
					return
				}
				if err := u.hub.Store.SetIcon(ident.ID, r.URI().Name(), data); err != nil {
					dialog.ShowError(err, u.win)
					return
				}
				u.reopenSettings()
			}, u.win)
			d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".ico", ".svg"}))
			d.Show()
		})
		clearIcon := widget.NewButton("Remove image", func() {
			_ = u.hub.Store.ClearIcon(ident.ID)
			u.reopenSettings()
		})
		notes := container.NewVBox()
		for i := range ident.Notes {
			i := i
			noteName := widget.NewEntry()
			noteName.SetPlaceHolder("Field")
			noteName.SetText(ident.Notes[i].Name)
			noteName.OnChanged = func(s string) {
				ident.Notes[i].Name = s
				save()
			}
			noteText := widget.NewMultiLineEntry()
			noteText.SetPlaceHolder("Note")
			noteText.SetText(ident.Notes[i].Text)
			noteText.OnChanged = func(s string) {
				ident.Notes[i].Text = s
				save()
			}
			drop := widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), func() {
				ident.Notes = append(ident.Notes[:i], ident.Notes[i+1:]...)
				save()
				u.reopenSettings()
			})
			notes.Add(container.NewBorder(nil, nil, nil, drop, noteName))
			notes.Add(noteText)
		}
		addNote := widget.NewButtonWithIcon("Add note", theme.ContentAddIcon(), func() {
			ident.Notes = append(ident.Notes, model.Note{})
			save()
			u.reopenSettings()
		})
		services := container.NewVBox()
		if len(u.hub.Store.Services()) == 0 {
			services.Add(hint("No services yet."))
		}
		for _, svc := range u.hub.Store.Services() {
			s := svc
			on := false
			for _, sid := range ident.Services {
				if sid == s.ID {
					on = true
				}
			}
			check := widget.NewCheck(serviceTitle(s), nil)
			check.SetChecked(on)
			check.OnChanged = func(v bool) {
				_ = u.hub.Store.AssignService(ident.ID, s.ID, v)
				u.reopenSettings()
			}
			services.Add(check)
		}
		out.Add(group("Profile",
			container.NewHBox(face, container.NewVBox(choose, clearIcon)),
			form(
				formLabel("Name"), name,
				formLabel("Username"), user,
				formLabel("Pronouns"), pronouns,
			),
			notes,
			addNote,
		))
		out.Add(group("Services", services))
		fp := widget.NewEntry()
		fp.SetText(ident.Public)
		fp.Disable()
		copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() { u.win.Clipboard().SetContent(ident.Public) })
		out.Add(group("Signing key", form(
			formLabel("Key prefix"), monoLabel(ident.Prefix()),
			formLabel("Public key"), container.NewBorder(nil, nil, nil, copyBtn, fp),
		)))
		confirm := widget.NewEntry()
		confirm.SetPlaceHolder(ident.Prefix())
		del := widget.NewButtonWithIcon("Delete identity", theme.DeleteIcon(), func() {
			if confirm.Text == ident.Prefix() {
				_ = u.hub.Store.DeleteIdentity(ident.ID)
				u.identityPick = ""
				u.reopenSettings()
			}
		})
		del.Importance = widget.DangerImportance
		del.Disable()
		confirm.OnChanged = func(s string) {
			if s == ident.Prefix() {
				del.Enable()
			} else {
				del.Disable()
			}
		}
		out.Add(group("Delete",
			hint("Deleting removes this profile and its signing key. Type the key prefix "+ident.Prefix()+" to confirm."),
			container.NewHBox(width(220, confirm), del),
		))
	}
	return out
}

func (u *UI) serviceSettings() fyne.CanvasObject {
	box := container.NewVBox()
	ids := u.identityChoices()
	for _, svc := range u.hub.Store.Services() {
		s := svc
		pluginSel := widget.NewSelect(u.pluginIDs(), nil)
		pluginSel.SetSelected(s.Plugin)
		pluginSel.OnChanged = func(p string) {
			s.Plugin = p
			_ = u.hub.Store.PutService(s)
			u.syncContextFromService(s)
			u.reopenSettings()
		}
		identSel := widget.NewSelect(ids.labels, nil)
		identSel.SetSelected(ids.label(s.IdentityID))
		identSel.OnChanged = func(label string) {
			s.IdentityID = ids.id(label)
			_ = u.hub.Store.PutService(s)
			u.syncContextFromService(s)
		}
		fields := form(
			formLabel("Plugin"), width(220, pluginSel),
			formLabel("Identity"), width(220, identSel),
		)
		if s.Config == nil {
			s.Config = map[string]string{}
		}
		u.addConnectionFields(fields, s.Plugin, s.Config, func() {
			_ = u.hub.Store.PutService(s)
			u.syncContextFromService(s)
		})
		status := u.hub.Status(s.ID)
		if status == "" {
			status = "Not connected"
		}
		conn := widget.NewButtonWithIcon("Connect", theme.MediaPlayIcon(), func() {
			if err := u.hub.Connect(s); err != nil {
				dialog.ShowError(err, u.win)
			}
			u.reopenSettings()
		})
		conn.Importance = widget.HighImportance
		disc := widget.NewButtonWithIcon("Disconnect", theme.MediaStopIcon(), func() {
			u.hub.Disconnect(s)
			u.reopenSettings()
		})
		box.Add(widget.NewCard(s.ID, status, container.NewVBox(fields, trailing(disc, conn))))
	}
	box.Add(container.NewHBox(widget.NewButtonWithIcon("Add service", theme.ContentAddIcon(), func() {
		id := "svc-" + modelID()
		ident := ""
		if len(ids.ids) > 0 {
			ident = ids.ids[0]
		}
		_ = u.hub.Store.PutService(model.Service{ID: id, Plugin: model.PluginLocal, IdentityID: ident, Config: map[string]string{}})
		u.reopenSettings()
	})))
	return box
}

func (u *UI) pluginSettings() fyne.CanvasObject {
	builtin := form()
	for _, p := range []struct{ name, id string }{
		{"local", model.PluginLocal},
		{"markdown-direct", model.PluginDirect},
		{"irc", model.PluginIRC},
		{"xmpp", model.PluginXMPP},
	} {
		marks := widget.NewLabel(strings.Join(u.hub.Caps(p.id).Names(), " · "))
		marks.Wrapping = fyne.TextWrapWord
		builtin.Add(widget.NewLabelWithStyle(p.name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		builtin.Add(marks)
	}
	external := container.NewVBox()
	cfg := u.hub.Store.Config()
	for _, ref := range cfg.Plugins {
		r := ref
		path := widget.NewEntry()
		path.SetText(r.Path)
		path.OnChanged = func(s string) {
			r.Path = s
			u.putPlugin(r)
		}
		en := widget.NewCheck("Enabled", nil)
		en.SetChecked(r.Enabled)
		en.OnChanged = func(on bool) {
			r.Enabled = on
			u.putPlugin(r)
		}
		reload := widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), func() {
			probed, err := u.probePlugin(r.Path)
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			r.ID = probed.ID
			r.Marks = probed.Marks
			r.Instructions = probed.Instructions
			r.Fields = probed.Fields
			u.putPlugin(r)
			u.reopenSettings()
		})
		marks := "no formatting"
		if len(r.Marks) > 0 {
			marks = strings.Join(r.Marks, " · ")
		}
		external.Add(container.NewVBox(
			container.NewBorder(nil, nil, width(160, boldLabel(r.ID)), container.NewHBox(en, reload), path),
			hint(marks),
		))
	}
	if len(cfg.Plugins) == 0 {
		external.Add(hint("No external plugins yet."))
	}
	addPath := widget.NewEntry()
	addPath.SetPlaceHolder("Path to plugin executable")
	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		probed, err := u.probePlugin(addPath.Text)
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		ref := model.PluginRef{
			ID: probed.ID, Path: addPath.Text, Enabled: true, Marks: probed.Marks,
			Instructions: probed.Instructions, Fields: probed.Fields,
		}
		u.putPlugin(ref)
		u.reopenSettings()
	})
	return container.NewVBox(
		group("Built-in", builtin),
		group("External", external, container.NewBorder(nil, nil, nil, addBtn, addPath)),
	)
}

func (u *UI) putPlugin(ref model.PluginRef) {
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
		for i := range c.Plugins {
			if c.Plugins[i].ID == ref.ID {
				c.Plugins[i] = ref
				return
			}
		}
		c.Plugins = append(c.Plugins, ref)
	})
}

func (u *UI) probePlugin(path string) (model.PluginRef, error) {
	if path == "" {
		return model.PluginRef{}, fmt.Errorf("empty plugin path")
	}
	p, err := plugin.Start(path, nil)
	if err != nil {
		return model.PluginRef{}, err
	}
	defer p.Stop()
	if err := p.Send(plugin.Message{Op: "capabilities"}); err != nil {
		return model.PluginRef{}, err
	}
	select {
	case m := <-p.Events():
		id := m.ID
		if id == "" {
			id = "plug-" + modelID()
		}
		return model.PluginRef{ID: id, Marks: m.Marks, Instructions: m.Instructions, Fields: m.Fields}, nil
	case <-time.After(2 * time.Second):
		return model.PluginRef{}, fmt.Errorf("no capabilities")
	}
}

func (u *UI) removeContext(id string) {
	emptyID := "ctx-" + modelID()
	used := false
	cfg := u.hub.Store.Config()
	for _, p := range layout.Panes(cfg.Layout) {
		if p.ContextID == id {
			p.ContextID = emptyID
			used = true
		}
	}
	if used {
		_ = u.hub.Store.PutContext(model.Context{ID: emptyID, Name: "Chat", Kind: model.KindConversation})
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Layout = cfg.Layout })
	}
	_ = u.hub.Store.DeleteContext(id)
}

func (u *UI) keySettings() fyne.CanvasObject {
	keys := form()
	for _, id := range u.hub.Store.Identities() {
		ident := id
		copyBtn := iconButton(theme.ContentCopyIcon(), func() { u.win.Clipboard().SetContent(ident.Public) })
		keys.Add(widget.NewLabelWithStyle(ident.Label(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		keys.Add(container.NewBorder(nil, nil, nil, copyBtn, monoLabel(ident.Public)))
	}
	pass := widget.NewPasswordEntry()
	confirm := widget.NewPasswordEntry()
	wrap := widget.NewButtonWithIcon("Wrap keys", theme.ConfirmIcon(), func() {
		if pass.Text == "" || pass.Text != confirm.Text {
			dialog.ShowInformation("Wrap keys", "Enter the same passphrase in both boxes.", u.win)
			return
		}
		if err := u.hub.Store.Wrap(pass.Text); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		dialog.ShowInformation("Wrap keys", "Private keys are now wrapped with the passphrase.", u.win)
	})
	wrap.Importance = widget.HighImportance
	clearBtn := widget.NewButtonWithIcon("Clear passphrase", theme.CancelIcon(), func() {
		if pass.Text == "" {
			dialog.ShowInformation("Clear passphrase", "Enter the current passphrase first.", u.win)
			return
		}
		if err := u.hub.Store.ClearWrap(pass.Text); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		dialog.ShowInformation("Clear passphrase", "Private keys are no longer wrapped.", u.win)
	})
	pins := container.NewVBox()
	for _, pin := range u.hub.Store.Pins() {
		p := pin
		remove := iconButton(theme.DeleteIcon(), func() {
			_ = u.hub.Store.DeletePin(p.Address)
			u.reopenSettings()
		})
		key := monoLabel(p.Key)
		key.Importance = widget.LowImportance
		pins.Add(container.NewBorder(nil, nil, width(180, boldLabel(p.Address)), remove, key))
	}
	if len(pins.Objects) == 0 {
		pins.Add(hint("No pinned peers."))
	}
	out := container.NewVBox(
		group("Public keys", keys),
		group("Passphrase",
			hint("Wrap encrypts the private keys on disk. To clear it, enter the current passphrase."),
			form(formLabel("Passphrase"), pass, formLabel("Confirm"), confirm),
			container.NewHBox(wrap, clearBtn),
		),
		group("Pinned peers", pins),
	)
	if hs := u.hub.Store.Config().Handshake; len(hs) > 0 {
		lines := form()
		for id, line := range hs {
			lines.Add(monoLabel(id))
			lines.Add(hint(line))
		}
		out.Add(group("Handshakes", lines))
	}
	return out
}

var bindingNames = map[string]string{
	"settings": "Open settings", "bold": "Bold", "italic": "Italic", "strike": "Strikethrough",
	"code": "Inline code", "code_block": "Code block", "link": "Link", "heading": "Heading",
	"quote": "Quote", "list": "List", "send": "Send message", "newline": "New line",
	"next": "Next pane", "previous": "Previous pane",
}

func (u *UI) bindingSettings() fyne.CanvasObject {
	keys := u.hub.Store.Config().Keys
	if keys == nil {
		keys = store.DefaultKeys()
	}
	rows := form()
	order := []string{"settings", "bold", "italic", "strike", "code", "code_block", "link", "heading", "quote", "list", "send", "newline", "next", "previous"}
	for _, name := range order {
		n := name
		current := monoLabel(keys[n])
		en := widget.NewCheck("On", nil)
		en.SetChecked(store.BindingOn(u.hub.Store.Config(), n))
		en.OnChanged = func(on bool) {
			name := n
			_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
				if c.Enabled == nil {
					c.Enabled = map[string]bool{}
				}
				c.Enabled[name] = on
			})
		}
		rec := widget.NewButtonWithIcon("Record", theme.MediaRecordIcon(), func() {
			u.recording = n
			u.reopenSettings()
		})
		if u.recording == n {
			current.SetText("Press a key…")
			current.Importance = widget.HighImportance
			rec.SetText("Recording")
			rec.Importance = widget.HighImportance
		}
		rows.Add(formLabel(bindingNames[n]))
		rows.Add(container.NewBorder(nil, nil, en, width(130, rec), current))
	}
	return container.NewVBox(
		hint("Turn a chord off to keep its button and drop the key. Send off means only the Send button sends. Click Record, then press the new chord. Escape keeps the previous chord."),
		width(560, rows),
	)
}

func paneIDs(n *model.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == "pane" {
		return []string{n.ID}
	}
	out := paneIDs(n.Leading)
	return append(out, paneIDs(n.Trailing)...)
}
