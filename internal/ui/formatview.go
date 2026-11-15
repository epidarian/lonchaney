package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/caps"
)

// hideTextTheme keeps the entry's caret math and selection, and hides the raw characters.
type hideTextTheme struct{ fyne.Theme }

func (h hideTextTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameForeground, theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return color.NRGBA{A: 0}
	default:
		return h.Theme.Color(n, v)
	}
}

// formatView draws the message as styled text. The entry underneath still takes typing.
type formatView struct {
	widget.BaseWidget
	entry *composer
}

func newFormatView(c *composer) *formatView {
	v := &formatView{entry: c}
	v.ExtendBaseWidget(v)
	return v
}

func (v *formatView) Cursor() desktop.Cursor { return desktop.TextCursor }

func pointerInside(p fyne.Position, s fyne.Size) bool {
	return p.X >= 0 && p.Y >= 0 && p.X <= s.Width && p.Y <= s.Height
}

func (v *formatView) MouseIn(*desktop.MouseEvent)    {}
func (v *formatView) MouseMoved(*desktop.MouseEvent) {}
func (v *formatView) MouseOut() {
	v.Refresh()
}

func (v *formatView) MouseDown(e *desktop.MouseEvent) {
	v.entry.MouseDown(e)
	v.Refresh()
}

func (v *formatView) MouseUp(e *desktop.MouseEvent) {
	v.entry.MouseUp(e)
	v.Refresh()
}

func (v *formatView) Dragged(e *fyne.DragEvent) {
	v.entry.Dragged(e)
	v.Refresh()
}

func (v *formatView) DragEnd() {
	v.entry.DragEnd()
	v.Refresh()
}

func (v *formatView) Tapped(e *fyne.PointEvent) {
	v.entry.Tapped(e)
	v.Refresh()
}

func (v *formatView) CreateRenderer() fyne.WidgetRenderer {
	return &formatRenderer{view: v}
}

type formatRenderer struct {
	view *formatView
	objs []fyne.CanvasObject
}

func (r *formatRenderer) Destroy() {}
func (r *formatRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		return nil
	}
	return r.objs
}
func (r *formatRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }
func (r *formatRenderer) Refresh()           { r.Layout(r.view.Size()) }
func (r *formatRenderer) Layout(s fyne.Size) {
	c := r.view.entry
	size := theme.TextSize()
	pad := theme.InnerPadding()
	lh := size + theme.Padding()/2
	maxW := s.Width - pad*2
	if maxW < size {
		maxW = size
	}
	rs := []rune(c.Text)
	var back, front []fyne.CanvasObject
	if len(rs) == 0 {
		ph := canvas.NewText("Message", theme.Color(theme.ColorNamePlaceHolder))
		ph.TextSize = size
		ph.Move(fyne.NewPos(pad, pad))
		front = append(front, ph)
	}
	x, y := pad, pad
	for i := 0; i < len(rs); {
		if rs[i] == '\n' {
			x = pad
			y += lh
			i++
			continue
		}
		j := i + 1
		style, strike := look(c.spans, i)
		for j < len(rs) && rs[j] != '\n' && sameLook(c.spans, i, j) {
			j++
		}
		chunk := string(rs[i:j])
		adv := fyne.MeasureText(chunk, size, fyne.TextStyle{}).Width
		if x > pad && x+adv > pad+maxW {
			x = pad
			y += lh
		}
		t := canvas.NewText(chunk, theme.Color(theme.ColorNameForeground))
		t.TextSize = size
		t.TextStyle = style
		t.Move(fyne.NewPos(x, y))
		t.Resize(t.MinSize())
		if c.pinA != c.pinB {
			a, b := c.pinA, c.pinB
			if a > b {
				a, b = b, a
			}
			from, to := i, j
			if a > from {
				from = a
			}
			if b < to {
				to = b
			}
			if from < to {
				pre := fyne.MeasureText(string(rs[i:from]), size, fyne.TextStyle{}).Width
				w := fyne.MeasureText(string(rs[from:to]), size, fyne.TextStyle{}).Width
				hi := canvas.NewRectangle(theme.Color(theme.ColorNameSelection))
				hi.Move(fyne.NewPos(x+pre, y))
				hi.Resize(fyne.NewSize(w, lh))
				back = append(back, hi)
			}
		}
		front = append(front, t)
		if strike {
			ln := canvas.NewLine(theme.Color(theme.ColorNameForeground))
			ln.Position1 = fyne.NewPos(x, y+lh*0.55)
			ln.Position2 = fyne.NewPos(x+adv, y+lh*0.55)
			ln.StrokeWidth = 1
			front = append(front, ln)
		}
		x += adv
		i = j
	}
	if c.focused {
		cp := c.CursorPosition()
		caret := canvas.NewRectangle(theme.Color(theme.ColorNameForeground))
		caret.Move(cp)
		caret.Resize(fyne.NewSize(1, lh))
		front = append(front, caret)
	}
	r.objs = append(back, front...)
}

func look(spans []span, i int) (fyne.TextStyle, bool) {
	var st fyne.TextStyle
	strike := false
	for _, s := range spans {
		if i < s.start || i >= s.end {
			continue
		}
		switch s.mark {
		case caps.Bold, caps.Heading:
			st.Bold = true
		case caps.Italic, caps.Quote:
			st.Italic = true
		case caps.Code, caps.CodeBlock:
			st.Monospace = true
		case caps.Strike:
			strike = true
		}
	}
	return st, strike
}

func sameLook(spans []span, a, b int) bool {
	sa, xa := look(spans, a)
	sb, xb := look(spans, b)
	return sa == sb && xa == xb
}
