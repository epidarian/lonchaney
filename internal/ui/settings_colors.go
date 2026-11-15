package ui

import (
	"fmt"
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/caps"
	"lonchan/internal/model"
	"lonchan/internal/skin"
)

func (u *UI) colorSettings() fyne.CanvasObject {
	spec := u.hub.Store.Config().Theme

	author := canvas.NewText("identity", theme.Color(theme.ColorNamePrimary))
	author.TextStyle = fyne.TextStyle{Bold: true}
	bold := canvas.NewText("bold", theme.Color(theme.ColorNameForeground))
	bold.TextStyle = fyne.TextStyle{Bold: true}
	heading := canvas.NewText("heading", theme.Color(theme.ColorNameForeground))
	heading.TextStyle = fyne.TextStyle{Bold: true}
	heading.TextSize = 22
	hover := newSwatch(theme.Color(theme.ColorNameHover), 64, 28)
	disabled := canvas.NewText("strike", theme.Color(theme.ColorNameDisabled))
	sample := container.NewHBox(author, bold, heading, hover, disabled)
	cardBg := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	cardBg.CornerRadius = theme.InputRadiusSize()
	card := container.NewStack(cardBg, container.NewPadded(container.New(vcenter{}, sample)))

	preset := widget.NewSelect([]string{"Light", "Dark", "Custom"}, nil)
	preset.SetSelected(spec.Preset)
	preset.OnChanged = func(name string) {
		if name == "Custom" {
			return
		}
		u.setPreset(name)
	}

	colors := container.NewVBox()
	order := []string{"background", "foreground", "primary", "disabled", "selection", "inputBackground", "separator", "hover", "error", "success"}
	for _, name := range order {
		hex := spec.Colors[name]
		sw := newSwatch(skin.ParseHex(hex), 28, 28)
		r, g, b := hexBytes(hex)
		re := widget.NewEntry()
		ge := widget.NewEntry()
		be := widget.NewEntry()
		re.SetPlaceHolder("R")
		ge.SetPlaceHolder("G")
		be.SetPlaceHolder("B")
		re.SetText(strconv.Itoa(int(r)))
		ge.SetText(strconv.Itoa(int(g)))
		be.SetText(strconv.Itoa(int(b)))
		hexL := monoLabel(fmt.Sprintf("#%02X%02X%02X", r, g, b))
		hexL.Importance = widget.LowImportance
		key := name
		apply := func(string) {
			rr, _ := strconv.Atoi(re.Text)
			gg, _ := strconv.Atoi(ge.Text)
			bb, _ := strconv.Atoi(be.Text)
			hex := fmt.Sprintf("#%02X%02X%02X", clamp(rr), clamp(gg), clamp(bb))
			col := skin.ParseHex(hex)
			_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
				if c.Theme.Colors == nil {
					c.Theme.Colors = map[string]string{}
				}
				c.Theme.Colors[key] = hex
				c.Theme.Preset = "Custom"
			})
			sw.SetColor(col)
			hexL.SetText(hex)
			switch key {
			case "primary":
				author.Color = col
				author.Refresh()
			case "foreground":
				bold.Color = col
				heading.Color = col
				bold.Refresh()
				heading.Refresh()
			case "hover":
				hover.SetColor(col)
			case "disabled":
				disabled.Color = col
				disabled.Refresh()
			}
			u.applyTheme()
		}
		re.OnChanged = apply
		ge.OnChanged = apply
		be.OnChanged = apply
		colors.Add(container.NewHBox(
			width(150, widget.NewLabel(name)),
			container.New(vcenter{}, sw),
			width(64, re), width(64, ge), width(64, be),
			width(90, hexL),
		))
	}

	scalePct := widget.NewLabel("")
	scale := widget.NewSlider(0.75, 2)
	scale.Step = 0.05
	scale.Value = spec.FontScale
	if scale.Value == 0 {
		scale.Value = 1
	}
	scalePct.SetText(fmt.Sprintf("%.0f%%", scale.Value*100))
	scale.OnChanged = func(v float64) {
		scalePct.SetText(fmt.Sprintf("%.0f%%", v*100))
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Theme.FontScale = v })
		u.applyTheme()
	}
	scale.OnChangeEnded = func(float64) { u.reopenSettings() }

	fonts := form()
	for _, style := range []string{"regular", "bold", "italic", "monospace"} {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Built-in font")
		entry.SetText(spec.Fonts[style])
		st := style
		entry.OnChanged = func(s string) {
			_ = u.hub.Store.UpdateConfig(func(c *model.Config) {
				if c.Theme.Fonts == nil {
					c.Theme.Fonts = map[string]string{}
				}
				c.Theme.Fonts[st] = s
			})
			u.applyTheme()
		}
		browse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
			d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
				if rc == nil || err != nil {
					return
				}
				entry.SetText(rc.URI().Path())
				_ = rc.Close()
			}, u.win)
			d.SetFilter(storage.NewExtensionFileFilter([]string{".ttf", ".otf"}))
			d.Show()
		})
		fonts.Add(formLabel(style))
		fonts.Add(container.NewBorder(nil, nil, nil, browse, entry))
	}

	marks := append([]string{}, spec.MarkOrder...)
	if len(marks) == 0 {
		for _, m := range caps.All() {
			marks = append(marks, string(m))
		}
	}
	order2 := container.NewVBox()
	for i, name := range marks {
		i := i
		move := func(to int) {
			o := append([]string{}, marks...)
			o[i], o[to] = o[to], o[i]
			_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Theme.MarkOrder = o })
			u.reopenSettings()
		}
		up := iconButton(theme.MoveUpIcon(), func() { move(i - 1) })
		down := iconButton(theme.MoveDownIcon(), func() { move(i + 1) })
		if i == 0 {
			up.Disable()
		}
		if i == len(marks)-1 {
			down.Disable()
		}
		order2.Add(container.NewBorder(nil, nil, nil, container.NewHBox(up, down), widget.NewLabel(name)))
	}

	return container.NewVBox(
		group("Theme",
			u.skinThemeNote(),
			form(formLabel("Preset"), width(200, preset)),
			card,
		),
		group("Colors",
			hint("Red, green, and blue from 0 to 255. Editing any value switches the preset to Custom."),
			colors,
		),
		group("Text",
			form(formLabel("Font scale"), container.NewBorder(nil, nil, nil, width(56, scalePct), scale)),
		),
		group("Fonts",
			hint("Leave a path empty to use the built-in font. TrueType and OpenType files work."),
			fonts,
		),
		group("Formatting toolbar",
			hint("The order of the tools in the toolbar that appears over the message box."),
			width(360, order2),
		),
	)
}

func hexBytes(hex string) (r, g, b uint8) {
	c := skin.ParseHex(hex)
	return c.R, c.G, c.B
}

func clamp(n int) int {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

type swatch struct {
	widget.BaseWidget
	col  color.Color
	w, h float32
}

func newSwatch(c color.Color, w, h float32) *swatch {
	s := &swatch{col: c, w: w, h: h}
	s.ExtendBaseWidget(s)
	return s
}

func (s *swatch) SetColor(c color.Color) {
	s.col = c
	s.Refresh()
}

func (s *swatch) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(s.col)
	r.StrokeColor = theme.Color(theme.ColorNameSeparator)
	r.StrokeWidth = 1
	r.CornerRadius = 4
	return &swatchRenderer{s: s, rect: r}
}

type swatchRenderer struct {
	s    *swatch
	rect *canvas.Rectangle
}

func (r *swatchRenderer) Destroy() {}
func (r *swatchRenderer) Layout(sz fyne.Size) {
	r.rect.Resize(sz)
}
func (r *swatchRenderer) MinSize() fyne.Size { return fyne.NewSize(r.s.w, r.s.h) }
func (r *swatchRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect}
}
func (r *swatchRenderer) Refresh() {
	r.rect.FillColor = r.s.col
	r.rect.Refresh()
}

// skinThemeNote says when the active skin's colors take precedence over this page.
func (u *UI) skinThemeNote() fyne.CanvasObject {
	cfg := u.hub.Store.Config()
	pack, _ := u.activePack()
	if pack.Theme == nil || cfg.IgnoreSkinTheme {
		return container.NewStack()
	}
	note := "The " + cfg.Skin + " skin sets its own colors, so values it sets win over this page."
	if cfg.Theme.Preset != "Custom" {
		note = "The " + cfg.Skin + " skin sets its own colors. Edit a value here to fill in what the skin leaves out."
	}
	open := widget.NewButtonWithIcon("Skin settings", theme.MediaPhotoIcon(), func() { u.openSettingsSection("Skins") })
	open.Importance = widget.LowImportance
	return container.NewBorder(nil, nil, nil, open, hint(note))
}
