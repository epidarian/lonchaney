package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Split is a two-pane split whose handle width and ratio are persisted.
type Split struct {
	widget.BaseWidget
	Horizontal bool
	Ratio      float64
	Handle     float32
	Leading    fyne.CanvasObject
	Trailing   fyne.CanvasObject
	OnChange   func(float64)
	Decor      fyne.CanvasObject
}

func NewSplit(horizontal bool, ratio float64, handle float32, leading, trailing fyne.CanvasObject, on func(float64)) *Split {
	if ratio <= 0.05 {
		ratio = 0.05
	}
	if ratio >= 0.95 {
		ratio = 0.95
	}
	if handle < 2 {
		handle = 2
	}
	s := &Split{Horizontal: horizontal, Ratio: ratio, Handle: handle, Leading: leading, Trailing: trailing, OnChange: on}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Split) CreateRenderer() fyne.WidgetRenderer {
	bar := canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
	h := &splitHandle{split: s}
	h.ExtendBaseWidget(h)
	return &splitRenderer{split: s, bar: bar, handle: h, objs: []fyne.CanvasObject{s.Leading, s.Trailing, bar, h}}
}

type splitRenderer struct {
	split  *Split
	bar    *canvas.Rectangle
	handle *splitHandle
	objs   []fyne.CanvasObject
}

func (r *splitRenderer) Destroy() {}
func (r *splitRenderer) Objects() []fyne.CanvasObject {
	r.objs[0] = r.split.Leading
	r.objs[1] = r.split.Trailing
	if r.split.Decor != nil {
		return []fyne.CanvasObject{r.split.Leading, r.split.Trailing, r.bar, r.split.Decor, r.handle}
	}
	return r.objs
}
func (r *splitRenderer) Layout(size fyne.Size) {
	handle := r.split.Handle
	if r.split.Horizontal {
		lead := (size.Width - handle) * float32(r.split.Ratio)
		r.split.Leading.Resize(fyne.NewSize(lead, size.Height))
		r.split.Leading.Move(fyne.NewPos(0, 0))
		r.split.Trailing.Resize(fyne.NewSize(size.Width-handle-lead, size.Height))
		r.split.Trailing.Move(fyne.NewPos(lead+handle, 0))
		r.bar.Resize(fyne.NewSize(handle, size.Height))
		r.bar.Move(fyne.NewPos(lead, 0))
		if r.split.Decor != nil {
			r.split.Decor.Resize(r.bar.Size())
			r.split.Decor.Move(r.bar.Position())
		}
		r.handle.Resize(r.bar.Size())
		r.handle.Move(r.bar.Position())
		return
	}
	lead := (size.Height - handle) * float32(r.split.Ratio)
	r.split.Leading.Resize(fyne.NewSize(size.Width, lead))
	r.split.Leading.Move(fyne.NewPos(0, 0))
	r.split.Trailing.Resize(fyne.NewSize(size.Width, size.Height-handle-lead))
	r.split.Trailing.Move(fyne.NewPos(0, lead+handle))
	r.bar.Resize(fyne.NewSize(size.Width, handle))
	r.bar.Move(fyne.NewPos(0, lead))
	if r.split.Decor != nil {
		r.split.Decor.Resize(r.bar.Size())
		r.split.Decor.Move(r.bar.Position())
	}
	r.handle.Resize(r.bar.Size())
	r.handle.Move(r.bar.Position())
}
func (r *splitRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, 40)
}
func (r *splitRenderer) Refresh() {
	r.bar.FillColor = theme.Color(theme.ColorNameSeparator)
	r.Layout(r.split.Size())
	r.bar.Refresh()
	canvas.Refresh(r.split)
}

type splitHandle struct {
	widget.BaseWidget
	split *Split
	last  float32
}

func (h *splitHandle) CreateRenderer() fyne.WidgetRenderer {
	return &handleRenderer{rect: canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})}
}

func (h *splitHandle) Dragged(e *fyne.DragEvent) {
	size := h.split.Size()
	if h.split.Horizontal {
		width := size.Width - h.split.Handle
		if width <= 0 {
			return
		}
		h.split.Ratio += float64(e.Dragged.DX / width)
	} else {
		height := size.Height - h.split.Handle
		if height <= 0 {
			return
		}
		h.split.Ratio += float64(e.Dragged.DY / height)
	}
	if h.split.Ratio < 0.05 {
		h.split.Ratio = 0.05
	}
	if h.split.Ratio > 0.95 {
		h.split.Ratio = 0.95
	}
	h.split.Refresh()
}

func (h *splitHandle) DragEnd() {
	h.last = 0
	if h.split.OnChange != nil {
		h.split.OnChange(h.split.Ratio)
	}
}

func (h *splitHandle) Cursor() desktop.Cursor {
	if h.split.Horizontal {
		return desktop.HResizeCursor
	}
	return desktop.VResizeCursor
}

func (h *splitHandle) MouseIn(*desktop.MouseEvent)    {}
func (h *splitHandle) MouseMoved(*desktop.MouseEvent) {}
func (h *splitHandle) MouseOut()                      {}

type handleRenderer struct{ rect *canvas.Rectangle }

func (r *handleRenderer) Destroy()                     {}
func (r *handleRenderer) Layout(s fyne.Size)           { r.rect.Resize(s) }
func (r *handleRenderer) MinSize() fyne.Size           { return fyne.NewSize(1, 1) }
func (r *handleRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *handleRenderer) Refresh()                     {}
