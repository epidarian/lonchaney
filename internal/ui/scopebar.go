package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	scopeMine  = "mine"
	scopeAll   = "all"
	scopeGroup = "group"
)

// scopeOf treats an empty or unknown value as every account.
func scopeOf(s string) string {
	switch s {
	case scopeMine, scopeGroup:
		return s
	default:
		return scopeAll
	}
}

type scopeChoice struct {
	id, label, tip string
}

func scopeChoices() []scopeChoice {
	return []scopeChoice{
		{scopeMine, "Mine", "Only the account of the focused chat"},
		{scopeAll, "All", "Every person and chat"},
		{scopeGroup, "Group", "Group by account, then by users, groups, and chats"},
	}
}

// scopeBar is one three-part switch. The current part is filled with the selection color.
type scopeBar struct {
	widget.BaseWidget
	value string
	set   func(string)
}

func newScopeBar(set func(string)) *scopeBar {
	b := &scopeBar{value: scopeAll, set: set}
	b.ExtendBaseWidget(b)
	return b
}

func (b *scopeBar) setValue(v string) {
	b.value = scopeOf(v)
	b.Refresh()
}

func (b *scopeBar) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	choices := scopeChoices()
	cells := make([]fyne.CanvasObject, len(choices))
	for i, c := range choices {
		cells[i] = newScopeCell(b, c)
	}
	return &scopeRenderer{bar: b, track: track, cells: cells}
}

type scopeRenderer struct {
	bar   *scopeBar
	track *canvas.Rectangle
	cells []fyne.CanvasObject
}

func (r *scopeRenderer) Destroy() {}

func (r *scopeRenderer) Objects() []fyne.CanvasObject {
	return append([]fyne.CanvasObject{r.track}, r.cells...)
}

func (r *scopeRenderer) MinSize() fyne.Size {
	h := theme.Size(theme.SizeNameCaptionText) + theme.Padding()*2 + 4
	return fyne.NewSize(36*3, h)
}

func (r *scopeRenderer) Layout(s fyne.Size) {
	r.track.Resize(s)
	r.track.CornerRadius = s.Height / 2
	inset := float32(2)
	gap := float32(2)
	inner := s.Subtract(fyne.NewSize(inset*2, inset*2))
	w := (inner.Width - gap*float32(len(r.cells)-1)) / float32(len(r.cells))
	for i, c := range r.cells {
		c.Move(fyne.NewPos(inset+float32(i)*(w+gap), inset))
		c.Resize(fyne.NewSize(w, inner.Height))
	}
}

func (r *scopeRenderer) Refresh() {
	r.track.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.track.Refresh()
	for _, c := range r.cells {
		c.Refresh()
	}
}

type scopeCell struct {
	widget.BaseWidget
	bar    *scopeBar
	choice scopeChoice
	hover  bool
}

func newScopeCell(bar *scopeBar, choice scopeChoice) *scopeCell {
	c := &scopeCell{bar: bar, choice: choice}
	c.ExtendBaseWidget(c)
	return c
}

func (c *scopeCell) ToolTip() string { return c.choice.tip }

func (c *scopeCell) Tapped(*fyne.PointEvent) {
	if scopeOf(c.bar.value) == c.choice.id {
		return
	}
	c.bar.value = c.choice.id
	if c.bar.set != nil {
		c.bar.set(c.choice.id)
	}
	c.bar.Refresh()
}

func (c *scopeCell) MouseIn(*desktop.MouseEvent) {
	c.hover = true
	c.Refresh()
}
func (c *scopeCell) MouseMoved(*desktop.MouseEvent) {}
func (c *scopeCell) MouseOut() {
	c.hover = false
	c.Refresh()
}
func (c *scopeCell) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (c *scopeCell) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	text := canvas.NewText(c.choice.label, theme.ColorForWidget(theme.ColorNameForeground, c))
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = theme.SizeForWidget(theme.SizeNameCaptionText, c)
	r := &scopeCellRenderer{cell: c, bg: bg, text: text}
	r.Refresh()
	return r
}

type scopeCellRenderer struct {
	cell *scopeCell
	bg   *canvas.Rectangle
	text *canvas.Text
}

func (r *scopeCellRenderer) Destroy() {}
func (r *scopeCellRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.text}
}
func (r *scopeCellRenderer) MinSize() fyne.Size {
	return fyne.NewSize(36, r.text.MinSize().Height+theme.Padding())
}

func (r *scopeCellRenderer) Layout(s fyne.Size) {
	r.paint()
	r.bg.Resize(s)
	r.bg.CornerRadius = s.Height / 2
	pad := theme.Padding()
	r.text.Text = fitText(r.cell.choice.label, s.Width-pad*2, r.text.TextSize, r.text.TextStyle)
	th := r.text.MinSize().Height
	r.text.Resize(fyne.NewSize(s.Width, th))
	r.text.Move(fyne.NewPos(0, (s.Height-th)/2))
}

func (r *scopeCellRenderer) Refresh() {
	r.paint()
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *scopeCellRenderer) paint() {
	on := scopeOf(r.cell.bar.value) == r.cell.choice.id
	r.text.TextStyle.Bold = on
	r.text.Color = theme.ColorForWidget(theme.ColorNameForeground, r.cell)
	switch {
	case on:
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNameSelection, r.cell)
	case r.cell.hover:
		r.bg.FillColor = theme.ColorForWidget(theme.ColorNameHover, r.cell)
	default:
		r.bg.FillColor = color.Transparent
	}
}
