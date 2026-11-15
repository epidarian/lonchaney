package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/model"
	"lonchan/internal/skin"
)

func (u *UI) skinSettings() fyne.CanvasObject {
	dir := filepath.Join(u.hub.Store.Dir(), "skins")
	_ = os.MkdirAll(dir, 0o755)
	names := []string{"plain"}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	cfg := u.hub.Store.Config()
	active := cfg.Skin
	sel := widget.NewSelect(names, nil)
	if active == "" {
		sel.SetSelected("plain")
	} else {
		sel.SetSelected(active)
	}
	sel.OnChanged = func(name string) {
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Skin = name })
		u.applyTheme()
		u.reopenSettings()
	}
	add := widget.NewButtonWithIcon("Add folder…", theme.FolderOpenIcon(), func() {
		d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri == nil || err != nil {
				return
			}
			dest := filepath.Join(dir, uri.Name())
			_ = copyDir(uri.Path(), dest)
			u.reopenSettings()
		}, u.win)
		d.Show()
	})
	var pack skin.Pack
	if active == "plain" || active == "" {
		pack = skin.Plain()
	} else if loaded, err := skin.Load(filepath.Join(dir, active)); err == nil {
		pack = loaded.Pack
	}
	slots := form()
	themed := 0
	for _, slot := range skin.Slots() {
		pane := pack.Panes[slot]
		if pane.Theme != nil {
			themed++
		}
		file := ""
		if active != "plain" && active != "" {
			file = skin.File(filepath.Join(dir, active), slot)
		}
		detail := mutedLabel(slotDetail(pane, file))
		detail.TextStyle.Monospace = true
		slots.Add(widget.NewLabelWithStyle(slot, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		slots.Add(detail)
	}

	useTheme := widget.NewCheck("Use the skin's colors, sizes, fonts, and icons", nil)
	useTheme.SetChecked(!cfg.IgnoreSkinTheme)
	useTheme.OnChanged = func(on bool) {
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.IgnoreSkinTheme = !on })
		u.applyTheme()
		u.reopenSettings()
	}
	styleNote := "This skin only draws backgrounds. Controls use Settings → Colors."
	if pack.Theme != nil || themed > 0 {
		styleNote = fmt.Sprintf("This skin restyles controls: %s.", themeSummary(pack.Theme, themed))
	} else {
		useTheme.Disable()
	}
	return container.NewVBox(
		group("Window skin",
			form(formLabel("Skin"), container.NewBorder(nil, nil, nil, add, sel)),
			hint("Add copies a folder into "+dir+"."),
		),
		group("Control style",
			hint(styleNote),
			useTheme,
			hint("Turn this off to keep your Settings → Colors theme with this skin's drawing."),
		),
		group("Slots",
			hint("What the selected skin draws for each slot: its image file, how many layers, where content goes, and whether it restyles controls."),
			slots,
		),
	)
}

// themeSummary says what a skin's theme blocks set, for the Skins page.
func themeSummary(t *skin.Theme, themedSlots int) string {
	var parts []string
	if t != nil {
		if t.Variant != "" {
			parts = append(parts, t.Variant+" variant")
		}
		if n := len(t.Colors); n > 0 {
			parts = append(parts, fmt.Sprintf("%d colors", n))
		}
		if n := len(t.Sizes); n > 0 {
			parts = append(parts, fmt.Sprintf("%d sizes", n))
		}
		if n := len(t.Fonts); n > 0 {
			parts = append(parts, fmt.Sprintf("%d fonts", n))
		}
		if n := len(t.Icons); n > 0 {
			parts = append(parts, fmt.Sprintf("%d icons", n))
		}
	}
	if themedSlots > 0 {
		parts = append(parts, fmt.Sprintf("%d slots with their own style", themedSlots))
	}
	if len(parts) == 0 {
		return "an empty theme block"
	}
	return strings.Join(parts, ", ")
}

func slotDetail(pane skin.PaneSkin, file string) string {
	imageName := "(none)"
	if file != "" {
		imageName = filepath.Base(file)
	}
	line := fmt.Sprintf("image %s  ·  layers %d", imageName, len(pane.Layers))
	if pane.Theme != nil {
		line += "  ·  styled"
	}
	if len(pane.Content) == 0 {
		return line + "  ·  content (none)"
	}
	keys := make([]string, 0, len(pane.Content))
	for k := range pane.Content {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := pane.Content[k]
		if len(r) < 4 {
			continue
		}
		line += fmt.Sprintf("  ·  %s [%.2f, %.2f, %.2f, %.2f]", k, r[0], r[1], r[2], r[3])
	}
	return line
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
