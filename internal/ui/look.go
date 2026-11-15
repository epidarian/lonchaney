package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynelayout "fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// pageTitle is the large heading at the top of a settings page.
func pageTitle(s string) *widget.Label {
	l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameHeadingText
	return l
}

// groupTitle heads a group of related controls inside a page.
func groupTitle(s string) *widget.Label {
	l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameSubHeadingText
	return l
}

// group is a titled block preceded by a separator.
func group(title string, body ...fyne.CanvasObject) fyne.CanvasObject {
	objs := []fyne.CanvasObject{widget.NewSeparator(), groupTitle(title)}
	return container.NewVBox(append(objs, body...)...)
}

// hint is muted help text. It truncates rather than wraps: a wrapping label in a
// VBox reports its height before it knows its width, which leaves gaps or overlaps.
func hint(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Importance = widget.LowImportance
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// truncLabel is a label that ellipsizes instead of growing its parent.
func truncLabel(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

func mutedLabel(s string) *widget.Label {
	l := truncLabel(s)
	l.Importance = widget.LowImportance
	return l
}

func boldLabel(s string) *widget.Label {
	l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

func monoLabel(s string) *widget.Label {
	l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// iconButton is a compact, low-importance button that shows only an icon.
func iconButton(icon fyne.Resource, tap func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tap)
	b.Importance = widget.LowImportance
	return b
}

// trailing puts objs at the right edge of a row.
func trailing(objs ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(append([]fyne.CanvasObject{fynelayout.NewSpacer()}, objs...)...)
}

// form lays out label/control pairs in two aligned columns.
func form(pairs ...fyne.CanvasObject) *fyne.Container {
	return container.New(fynelayout.NewFormLayout(), pairs...)
}

// formLabel is the left-column label of a form row.
func formLabel(s string) *widget.Label {
	return widget.NewLabelWithStyle(s, fyne.TextAlignTrailing, fyne.TextStyle{})
}

// inset pads a page body.
func inset(o fyne.CanvasObject) fyne.CanvasObject {
	p := theme.Padding()
	return container.New(fynelayout.NewCustomPaddedLayout(p, p*3, p*3, p*3), o)
}

// width fixes the width of o and keeps it at the leading edge of whatever holds it.
func width(w float32, o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(container.New(fixedW{w: w}, o))
}

type fixedW struct{ w float32 }

func (f fixedW) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objs {
		if m := o.MinSize().Height; m > h {
			h = m
		}
	}
	return fyne.NewSize(f.w, h)
}

func (f fixedW) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

// height fixes the height of o.
func height(h float32, o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(fixedH{h: h}, o)
}

// vcenter keeps its child at minimum height, full width, centered vertically.
type vcenter struct{}

func (vcenter) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}

func (vcenter) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		h := o.MinSize().Height
		if h > size.Height {
			h = size.Height
		}
		o.Resize(fyne.NewSize(size.Width, h))
		o.Move(fyne.NewPos(0, (size.Height-h)/2))
	}
}

// fitText shortens s with an ellipsis until it fits w at the given text size and style.
func fitText(s string, w, size float32, style fyne.TextStyle) string {
	if w <= 0 {
		return ""
	}
	if fyne.MeasureText(s, size, style).Width <= w {
		return s
	}
	r := []rune(s)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fyne.MeasureText(string(r[:mid])+"…", size, style).Width <= w {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ""
	}
	return string(r[:lo]) + "…"
}

// showMenuBelow pops a menu up under anchor.
func (u *UI) showMenuBelow(m *fyne.Menu, anchor fyne.CanvasObject) {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	pos.Y += anchor.Size().Height
	widget.ShowPopUpMenuAtPosition(m, u.win.Canvas(), pos)
}

// keepScope gives children that w builds after a skin theme override was
// applied the same override. Fyne applies an override once, to the objects
// that exist at that moment.
func keepScope(w fyne.Widget, child fyne.CanvasObject) fyne.CanvasObject {
	th := theme.CurrentForWidget(w)
	if th == fyne.CurrentApp().Settings().Theme() {
		return child
	}
	return container.NewThemeOverride(child, th)
}
