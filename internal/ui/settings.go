package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/model"
)

var sectionIcons = map[string]fyne.Resource{
	"Layout":      theme.GridIcon(),
	"Colors":      theme.ColorPaletteIcon(),
	"Skins":       theme.MediaPhotoIcon(),
	"Contexts":    theme.ListIcon(),
	"Identities":  theme.AccountIcon(),
	"Services":    theme.StorageIcon(),
	"Plugins":     theme.DocumentIcon(),
	"Keys":        theme.LoginIcon(),
	"Keybindings": theme.ComputerIcon(),
}

var sectionBlurbs = map[string]string{
	"Layout":      "Arrange the panes of the chat window.",
	"Colors":      "Theme colors, text size, fonts, and the formatting toolbar.",
	"Skins":       "Drawing packs for the window and its panes.",
	"Contexts":    "A service connection for an account, the plugin fields that open it, and the people added on it.",
	"Identities":  "Profiles you speak as. Each one keeps a signing key for direct messages.",
	"Services":    "The connections each identity uses to reach people.",
	"Plugins":     "Protocol plugins and the formatting each one carries.",
	"Keys":        "Public keys, passphrase protection, and pinned peers.",
	"Keybindings": "Keyboard chords for formatting, sending, and moving between panes.",
}

func (u *UI) settingsRoot() fyne.CanvasObject {
	names := settingsSections
	body := container.NewStack()
	if u.settingsOff == nil {
		u.settingsOff = map[string]fyne.Position{}
	}
	show := func(name string) {
		u.settingsSection = name
		var content fyne.CanvasObject
		switch name {
		case "Layout":
			content = u.layoutSettings()
		case "Colors":
			content = u.colorSettings()
		case "Skins":
			content = u.skinSettings()
		case "Contexts":
			content = u.contextSettings()
		case "Identities":
			content = u.identitySettings()
		case "Services":
			content = u.serviceSettings()
		case "Plugins":
			content = u.pluginSettings()
		case "Keys":
			content = u.keySettings()
		case "Keybindings":
			content = u.bindingSettings()
		}
		page := container.NewVBox(pageTitle(name), hint(sectionBlurbs[name]), content)
		scroll := container.NewVScroll(inset(page))
		if off, ok := u.settingsOff[name]; ok {
			scroll.Offset = off
		}
		scroll.OnScrolled = func(p fyne.Position) { u.settingsOff[name] = p }
		body.Objects = []fyne.CanvasObject{scroll}
		body.Refresh()
	}
	list := widget.NewList(
		func() int { return len(names) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.SettingsIcon()), widget.NewLabel(""))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*fyne.Container)
			row.Objects[0].(*widget.Icon).SetResource(sectionIcons[names[id]])
			row.Objects[1].(*widget.Label).SetText(names[id])
		},
	)
	list.HideSeparators = true
	list.OnSelected = func(id widget.ListItemID) {
		if names[id] != u.settingsSection {
			show(names[id])
			u.setMenu()
		}
	}
	back := widget.NewButtonWithIcon("Back to chat", theme.NavigateBackIcon(), u.closeSettings)
	title := groupTitle("Settings")
	nav := container.NewBorder(container.NewPadded(title), container.NewPadded(back), nil, nil, list)
	cfg := u.hub.Store.Config()
	ratio := cfg.SettingsRatio
	if ratio <= 0 {
		ratio = 0.20
	}
	pack, dir := u.activePack()
	split := NewSplit(true, ratio, float32(cfg.SplitHandlePx), u.newFrame(pack, dir, "settings-nav", nil, nav), u.newFrame(pack, dir, "settings", nil, body), func(r float64) {
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.SettingsRatio = r })
	})
	start := u.settingsSection
	if start == "" {
		start = "Layout"
	}
	show(start)
	for i, name := range names {
		if name == start {
			list.Select(i)
		}
	}
	return split
}

// openSettings opens Settings on the page last shown, or Layout.
func (u *UI) openSettings() {
	name := u.settingsSection
	if name == "" {
		name = "Layout"
	}
	u.openSettingsSection(name)
}

// reopenSettings rebuilds the current settings page after a change.
func (u *UI) reopenSettings() { u.openSettings() }

func (u *UI) openSettingsSection(name string) {
	u.hideSuite()
	u.settingsSection = name
	u.inSettings = true
	u.win.SetContent(u.settingsRoot())
	u.setMenu()
}

func (u *UI) closeSettings() {
	u.inSettings = false
	u.win.SetContent(u.shellRoot())
	u.retitle()
	u.setMenu()
}
