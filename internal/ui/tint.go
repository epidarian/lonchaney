package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The canvas primitives read the app theme. These wrappers read the theme of
// wherever they are placed, so a skin slot's theme block reaches them.

// tintRect is a rectangle filled with a theme color, optionally faded.
type tintRect struct {
	widget.BaseWidget
	name   fyne.ThemeColorName
	alpha  uint8 // 0 keeps the theme color's own opacity
	radius float32
}

func newTintRect(name fyne.ThemeColorName, alpha uint8) *tintRect {
	t := &tintRect{name: name, alpha: alpha}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tintRect) CreateRenderer() fyne.WidgetRenderer {
	r := &tintRectRenderer{t: t, rect: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type tintRectRenderer struct {
	t    *tintRect
	rect *canvas.Rectangle
}

func (r *tintRectRenderer) Destroy()                     {}
func (r *tintRectRenderer) Layout(s fyne.Size)           { r.rect.Resize(s) }
func (r *tintRectRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *tintRectRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *tintRectRenderer) Refresh() {
	c := theme.ColorForWidget(r.t.name, r.t)
	if r.t.alpha != 0 {
		c = fade(c, r.t.alpha)
	}
	r.rect.FillColor = c
	r.rect.CornerRadius = r.t.radius
	r.rect.Refresh()
}

// tintText is one line of text in a theme color, inset like a Label.
type tintText struct {
	widget.BaseWidget
	text  string
	name  fyne.ThemeColorName
	style fyne.TextStyle
}

func newTintText(text string, name fyne.ThemeColorName) *tintText {
	t := &tintText{text: text, name: name}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tintText) CreateRenderer() fyne.WidgetRenderer {
	r := &tintTextRenderer{t: t, text: canvas.NewText(t.text, color.Transparent)}
	r.Refresh()
	return r
}

type tintTextRenderer struct {
	t    *tintText
	text *canvas.Text
}

func (r *tintTextRenderer) pad() float32 { return theme.SizeForWidget(theme.SizeNameInnerPadding, r.t) }
func (r *tintTextRenderer) Destroy()     {}
func (r *tintTextRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.text}
}
func (r *tintTextRenderer) MinSize() fyne.Size {
	m := r.text.MinSize()
	return fyne.NewSize(m.Width+r.pad()*2, m.Height)
}
func (r *tintTextRenderer) Layout(s fyne.Size) {
	r.text.Move(fyne.NewPos(r.pad(), (s.Height-r.text.MinSize().Height)/2))
	r.text.Resize(fyne.NewSize(s.Width-r.pad()*2, r.text.MinSize().Height))
}
func (r *tintTextRenderer) Refresh() {
	r.text.Text = r.t.text
	r.text.Color = theme.ColorForWidget(r.t.name, r.t)
	r.text.TextSize = theme.SizeForWidget(theme.SizeNameText, r.t)
	r.text.TextStyle = r.t.style
	r.text.Refresh()
}
