package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"lonchan/internal/caps"
	"lonchan/internal/layout"
	"lonchan/internal/model"
	"lonchan/internal/store"
)

var settingsSections = []string{"Layout", "Colors", "Skins", "Contexts", "Identities", "Services", "Plugins", "Keys", "Keybindings"}

// markShortcuts are the format chords; bindShortcuts and the Format menu share them.
var markShortcuts = []struct {
	mark  caps.Mark
	label string
	key   fyne.KeyName
	mod   fyne.KeyModifier
	group int
}{
	{caps.Bold, "Bold", fyne.KeyB, fyne.KeyModifierControl, 0},
	{caps.Italic, "Italic", fyne.KeyI, fyne.KeyModifierControl, 0},
	{caps.Strike, "Strikethrough", fyne.KeyX, fyne.KeyModifierControl | fyne.KeyModifierShift, 0},
	{caps.Code, "Inline Code", fyne.KeyE, fyne.KeyModifierControl, 1},
	{caps.CodeBlock, "Code Block", fyne.KeyE, fyne.KeyModifierControl | fyne.KeyModifierShift, 1},
	{caps.Link, "Link", fyne.KeyK, fyne.KeyModifierControl, 2},
	{caps.Heading, "Heading", fyne.KeyH, fyne.KeyModifierControl, 2},
	{caps.Quote, "Quote", fyne.KeyQ, fyne.KeyModifierControl | fyne.KeyModifierShift, 2},
	{caps.List, "List", fyne.KeyL, fyne.KeyModifierControl, 2},
}

func chord(key fyne.KeyName, mod fyne.KeyModifier) fyne.Shortcut {
	return &desktop.CustomShortcut{KeyName: key, Modifier: mod}
}

func (u *UI) setMenu() {
	sep := fyne.NewMenuItemSeparator

	newIdentity := fyne.NewMenuItem("New Identity", func() {
		if id, err := u.hub.Store.AddIdentity(""); err == nil {
			u.identityPick = id.ID
		}
		u.openSettingsSection("Identities")
	})
	file := fyne.NewMenu("File",
		fyne.NewMenuItem("New User…", func() { u.addTarget("user") }),
		fyne.NewMenuItem("New Group…", func() { u.addTarget("group") }),
		newIdentity,
		sep(),
		fyne.NewMenuItem("Quit", nil),
	)

	focused := u.focusedConversation()
	svc, _ := u.hub.Store.Service(focused.ServiceID)
	have := u.hub.Caps(svc.Plugin)
	var fmtItems []*fyne.MenuItem
	for i, ms := range markShortcuts {
		if i > 0 && markShortcuts[i-1].group != ms.group {
			fmtItems = append(fmtItems, sep())
		}
		mark := ms.mark
		it := fyne.NewMenuItem(ms.label, func() { u.applyMark(mark) })
		it.Shortcut = chord(ms.key, ms.mod)
		it.Disabled = u.inSettings || focused.ID == "" || !have.Has(mark)
		fmtItems = append(fmtItems, it)
	}
	format := fyne.NewMenu("Format", fmtItems...)

	preset := u.hub.Store.Config().Theme.Preset
	next := fyne.NewMenuItem("Next Pane", func() { u.cyclePane(1) })
	next.Shortcut = chord(fyne.KeyTab, fyne.KeyModifierControl)
	next.Disabled = u.inSettings
	prev := fyne.NewMenuItem("Previous Pane", func() { u.cyclePane(-1) })
	prev.Shortcut = chord(fyne.KeyTab, fyne.KeyModifierControl|fyne.KeyModifierShift)
	prev.Disabled = u.inSettings
	light := fyne.NewMenuItem("Light Theme", func() { u.setPreset("Light") })
	light.Checked = preset == "Light" || preset == ""
	dark := fyne.NewMenuItem("Dark Theme", func() { u.setPreset("Dark") })
	dark.Checked = preset == "Dark"
	panesWin := fyne.NewMenuItem("Panes in Separate Window", func() {
		if u.panesWin != nil {
			u.dockPanes()
			return
		}
		u.popOutPanes()
	})
	panesWin.Checked = u.panesWin != nil
	view := fyne.NewMenu("View",
		next, prev,
		sep(),
		panesWin,
		sep(),
		light, dark,
		sep(),
		fyne.NewMenuItem("Reset Layout (10 | 80 | 10)", func() {
			u.resetLayout()
			if u.inSettings {
				u.reopenSettings()
				return
			}
			u.refresh()
		}),
	)

	// The Pane menu acts on the active chat pane, and its title says which one.
	paneTitle := "Pane"
	paneItems := u.paneItems(layout.FindPane(u.hub.Store.Config().Layout, u.focused))
	if n := u.paneNumber(u.focused); n > 0 && u.isConversationPane(u.focused) {
		paneTitle = "Pane " + itoa(n)
	}
	if len(paneItems) == 0 {
		none := fyne.NewMenuItem("No chat pane", nil)
		none.Disabled = true
		paneItems = append(paneItems, none)
	}
	pane := fyne.NewMenu(paneTitle, paneItems...)

	open := fyne.NewMenuItem("Open Settings", u.openSettings)
	open.Shortcut = chord(fyne.KeyComma, fyne.KeyModifierControl)
	settingsItems := []*fyne.MenuItem{open, sep()}
	for _, name := range settingsSections {
		section := name
		it := fyne.NewMenuItem(section, func() { u.openSettingsSection(section) })
		it.Checked = u.inSettings && u.settingsSection == section
		settingsItems = append(settingsItems, it)
	}
	back := fyne.NewMenuItem("Back to Chat", u.closeSettings)
	back.Disabled = !u.inSettings
	settingsItems = append(settingsItems, sep(), back)

	u.win.SetMainMenu(fyne.NewMainMenu(file, format, view, pane, fyne.NewMenu("Settings", settingsItems...)))
}

// paneItems is a pane's menu: what it shows, the chat's identity, splitting,
// closing, and its header. The pane header's ⋮ button and the Pane menu use it.
func (u *UI) paneItems(pane *model.Node) []*fyne.MenuItem {
	if pane == nil {
		return nil
	}
	paneID := pane.ID
	showIn := fyne.NewMenuItem("Show in This Pane", nil)
	showIn.ChildMenu = fyne.NewMenu("", u.showItems(pane)...)
	items := []*fyne.MenuItem{showIn}

	if ctx, ok := u.hub.Store.Context(pane.ContextID); ok && ctx.Kind == model.KindConversation {
		var ids []*fyne.MenuItem
		for _, id := range u.hub.Store.Identities() {
			ident := id
			it := fyne.NewMenuItem(ident.Label(), func() { u.setIdentity(ctx, ident.ID) })
			it.Checked = ident.ID == ctx.IdentityID
			ids = append(ids, it)
		}
		if len(ids) > 0 {
			idItem := fyne.NewMenuItem("Chat As", nil)
			idItem.ChildMenu = fyne.NewMenu("", ids...)
			items = append(items, idItem)
		}
	}

	split := func(dir string) func() {
		return func() {
			if id := u.splitPane(paneID, dir, ""); id != "" {
				u.focusPane(id)
				if u.inSettings {
					u.reopenSettings()
					return
				}
				u.refresh()
			}
		}
	}
	closeItem := fyne.NewMenuItem("Close Pane", func() { u.closePane(paneID) })
	closeItem.Disabled = len(layout.Panes(u.hub.Store.Config().Layout)) < 2
	shown := pane.ShowHeader()
	header := fyne.NewMenuItem("Show Header", func() { u.setHeader(paneID, !shown) })
	header.Checked = shown
	return append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Split Right", split("h")),
		fyne.NewMenuItem("Split Down", split("v")),
		closeItem,
		fyne.NewMenuItemSeparator(),
		header,
	)
}

func (u *UI) setHeader(paneID string, on bool) {
	cfg := u.hub.Store.Config()
	if p := layout.FindPane(cfg.Layout, paneID); p != nil {
		p.HeaderVisible = model.BoolPtr(on)
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Layout = cfg.Layout })
	}
	if u.inSettings {
		u.reopenSettings()
		return
	}
	u.refresh()
}

// setPreset swaps the color preset and keeps fonts, scale, and mark order.
func (u *UI) setPreset(name string) {
	_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
		fresh := store.DefaultTheme(name)
		c.Theme.Preset = fresh.Preset
		c.Theme.Colors = fresh.Colors
		if c.Theme.FontScale <= 0 {
			c.Theme.FontScale = 1
		}
		if len(c.Theme.MarkOrder) == 0 {
			c.Theme.MarkOrder = fresh.MarkOrder
		}
	})
	u.applyTheme()
	if u.inSettings {
		u.reopenSettings()
		return
	}
	u.refresh()
}

// showItems are the things a pane can show: each chat, an empty chat, and the
// tool views. The pane title's navigator and "Show in This Pane" use them.
func (u *UI) showItems(pane *model.Node) []*fyne.MenuItem {
	paneID := pane.ID
	show := func(contextID string) func() {
		return func() {
			u.mountInPane(paneID, contextID)
			u.focusPane(paneID)
			if u.inSettings {
				u.closeSettings()
				return
			}
			u.refresh()
			u.focusComposer(paneID)
		}
	}
	var chats, tools []*fyne.MenuItem
	for _, c := range u.hub.Store.Contexts() {
		ctx := c
		label := ctx.Name
		if label == "" {
			label = ctx.ID
		}
		if ctx.Kind == model.KindConversation {
			if ctx.Target == "" {
				continue
			}
			label = ctx.Target
			if id, ok := u.hub.Store.Identity(ctx.IdentityID); ok {
				label += " (as " + id.Label() + ")"
			}
			if where := u.panesShowing(ctx.ID); where != "" && pane.ContextID != ctx.ID {
				label += " — in " + where
			}
		}
		it := fyne.NewMenuItem(label, show(ctx.ID))
		it.Checked = pane.ContextID == ctx.ID
		if ctx.Kind == model.KindConversation {
			chats = append(chats, it)
		} else {
			tools = append(tools, it)
		}
	}
	empty := fyne.NewMenuItem("Empty Chat", func() { show(u.emptyChat())() })
	if ctx, ok := u.hub.Store.Context(pane.ContextID); ok && ctx.Kind == model.KindConversation && ctx.Target == "" {
		empty.Checked = true
	}
	items := append(chats, fyne.NewMenuItemSeparator(), empty)
	if len(tools) > 0 {
		items = append(append(items, fyne.NewMenuItemSeparator()), tools...)
	}
	return items
}
