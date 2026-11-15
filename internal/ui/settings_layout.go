package ui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/layout"
	"lonchan/internal/model"
)

// choices maps the labels a Select shows to the ids it stores.
type choices struct {
	labels []string
	ids    []string
}

func (c choices) label(id string) string {
	for i, v := range c.ids {
		if v == id {
			return c.labels[i]
		}
	}
	return ""
}

func (c choices) id(label string) string {
	for i, v := range c.labels {
		if v == label {
			return c.ids[i]
		}
	}
	return ""
}

func (u *UI) contextChoices() choices {
	var c choices
	seen := map[string]int{}
	for _, ctx := range u.hub.Store.Contexts() {
		seen[ctx.Name]++
	}
	for _, ctx := range u.hub.Store.Contexts() {
		label := ctx.Name
		if label == "" || seen[ctx.Name] > 1 {
			label = ctx.Name + " (" + ctx.ID + ")"
		}
		c.labels = append(c.labels, label)
		c.ids = append(c.ids, ctx.ID)
	}
	return c
}

func identityChoice(id model.Identity) string {
	label := id.Label()
	if id.Username != "" && id.Username != label {
		label = label + " (@" + id.Username + ")"
	}
	if label == "" {
		return id.Prefix()
	}
	return label
}

func (u *UI) identityChoices() choices {
	var c choices
	for _, id := range u.hub.Store.Identities() {
		c.labels = append(c.labels, identityChoice(id))
		c.ids = append(c.ids, id.ID)
	}
	return c
}

func (u *UI) skinChoices() []string {
	skins := []string{"window default", "plain"}
	if entries, err := os.ReadDir(filepath.Join(u.hub.Store.Dir(), "skins")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				skins = append(skins, e.Name())
			}
		}
	}
	return skins
}

func (u *UI) layoutSettings() fyne.CanvasObject {
	cfg := u.hub.Store.Config()
	save := func() { _ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.Layout = cfg.Layout }) }
	ctxs := u.contextChoices()
	skins := u.skinChoices()
	newPane := func() *model.Node {
		return &model.Node{Kind: "pane", ID: "pane-" + u.freshID(), ContextID: "ctx-chat", HeaderVisible: model.BoolPtr(true)}
	}
	tree := container.NewVBox()
	var walk func(n *model.Node, depth int)
	walk = func(n *model.Node, depth int) {
		if n == nil {
			return
		}
		if n.Kind == "split" {
			dir := widget.NewSelect([]string{"Side by side", "Stacked"}, nil)
			if n.Dir == "v" {
				dir.SetSelected("Stacked")
			} else {
				dir.SetSelected("Side by side")
			}
			dir.OnChanged = func(s string) {
				if s == "Stacked" {
					n.Dir = "v"
				} else {
					n.Dir = "h"
				}
				save()
				u.reopenSettings()
			}
			pct := widget.NewLabel(fmt.Sprintf("%.0f%%", n.Ratio*100))
			ratio := widget.NewSlider(0.05, 0.95)
			ratio.Step = 0.01
			ratio.Value = n.Ratio
			ratio.OnChanged = func(v float64) {
				n.Ratio = v
				pct.SetText(fmt.Sprintf("%.0f%%", v*100))
			}
			ratio.OnChangeEnded = func(float64) {
				save()
				u.reopenSettings()
			}
			var more *widget.Button
			more = iconButton(theme.MoreHorizontalIcon(), func() {
				u.showMenuBelow(fyne.NewMenu("",
					fyne.NewMenuItem("Divide leading", func() {
						layout.DivideLeading(n, newPane())
						_ = u.hub.Store.Save()
						u.reopenSettings()
					}),
					fyne.NewMenuItem("Divide trailing", func() {
						layout.DivideTrailing(n, newPane())
						_ = u.hub.Store.Save()
						u.reopenSettings()
					}),
					fyne.NewMenuItemSeparator(),
					fyne.NewMenuItem("Remove split", func() {
						cfg.Layout = layout.RemoveSplit(cfg.Layout, n)
						save()
						u.reopenSettings()
					}),
				), more)
			})
			left := container.NewHBox(widget.NewIcon(theme.ViewRestoreIcon()), widget.NewLabelWithStyle("Split", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), width(150, dir))
			right := container.NewHBox(width(52, pct), more)
			tree.Add(indentRow(depth, container.NewBorder(nil, nil, left, right, ratio)))
			walk(n.Leading, depth+1)
			walk(n.Trailing, depth+1)
			return
		}
		sel := widget.NewSelect(ctxs.labels, nil)
		sel.SetSelected(ctxs.label(n.ContextID))
		sel.OnChanged = func(label string) {
			n.ContextID = ctxs.id(label)
			save()
		}
		head := widget.NewCheck("Header", nil)
		head.SetChecked(n.ShowHeader())
		head.OnChanged = func(on bool) {
			n.HeaderVisible = model.BoolPtr(on)
			save()
		}
		skinSel := widget.NewSelect(skins, nil)
		if n.SkinID == "" {
			skinSel.SetSelected("window default")
		} else {
			skinSel.SetSelected(n.SkinID)
		}
		skinSel.OnChanged = func(id string) {
			if id == "window default" {
				id = ""
			}
			n.SkinID = id
			save()
		}
		name := boldLabel("Pane " + itoa(u.paneNumber(n.ID)))
		if n.ID == u.layoutPick {
			name.Importance = widget.HighImportance
		}
		right := container.NewHBox(head, width(160, skinSel))
		if n.ID != "" {
			node := n
			var more *widget.Button
			more = iconButton(theme.MoreHorizontalIcon(), func() {
				u.showMenuBelow(fyne.NewMenu("", fyne.NewMenuItem("Remove pane", func() {
					cfg.Layout = layout.RemovePane(cfg.Layout, node.ID)
					save()
					u.reopenSettings()
				})), more)
			})
			right.Add(more)
		}
		left := container.NewHBox(widget.NewIcon(theme.FileIcon()), width(130, name))
		tree.Add(indentRow(depth, container.NewBorder(nil, nil, left, right, sel)))
	}
	walk(cfg.Layout, 0)

	handle := widget.NewEntry()
	handle.SetText(strconv.Itoa(int(cfg.SplitHandlePx)))
	handle.OnChanged = func(s string) {
		n, _ := strconv.Atoi(s)
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.SplitHandlePx = float64(n) })
	}
	navPct := widget.NewLabel(fmt.Sprintf("%.0f%%", cfg.SettingsRatio*100))
	nav := widget.NewSlider(0.15, 0.40)
	nav.Step = 0.01
	nav.Value = cfg.SettingsRatio
	nav.OnChanged = func(v float64) {
		navPct.SetText(fmt.Sprintf("%.0f%%", v*100))
		_ = u.hub.Store.UpdateConfig(func(c *model.Config) { c.SettingsRatio = v })
	}
	nav.OnChangeEnded = func(float64) { u.reopenSettings() }
	reset := widget.NewButtonWithIcon("Reset to 10 | 80 | 10", theme.ViewRefreshIcon(), func() {
		u.resetLayout()
		u.reopenSettings()
	})

	return container.NewVBox(
		group("Preview",
			height(180, u.layoutPreview(cfg.Layout)),
			hint("Click a pane to find it in the list below."),
		),
		group("Panes", tree),
		group("General",
			form(
				formLabel("Split handle"), container.NewHBox(width(72, handle), widget.NewLabel("px")),
				formLabel("Settings sidebar"), container.NewBorder(nil, nil, nil, width(52, navPct), nav),
			),
			container.NewHBox(reset),
		),
	)
}

func (u *UI) layoutPreview(n *model.Node) fyne.CanvasObject {
	if n == nil {
		return widget.NewLabel("")
	}
	if n.Kind == "pane" {
		rect := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
		rect.CornerRadius = theme.InputRadiusSize()
		if n.ID == u.layoutPick {
			rect.StrokeColor = theme.Color(theme.ColorNamePrimary)
		} else {
			rect.StrokeColor = theme.Color(theme.ColorNameSeparator)
		}
		rect.StrokeWidth = 2
		id := n.ID
		ctxName := n.ContextID
		if ctx, ok := u.hub.Store.Context(n.ContextID); ok && ctx.Name != "" {
			ctxName = ctx.Name
		}
		text := widget.NewLabelWithStyle(ctxName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		text.Truncation = fyne.TextTruncateEllipsis
		sub := widget.NewLabelWithStyle("Pane "+itoa(u.paneNumber(id)), fyne.TextAlignCenter, fyne.TextStyle{})
		sub.Importance = widget.LowImportance
		sub.Truncation = fyne.TextTruncateEllipsis
		return newPreviewHit(container.NewStack(rect, container.New(vcenter{}, container.NewVBox(text, sub))), func() {
			u.layoutPick = id
			u.reopenSettings()
		})
	}
	lead := u.layoutPreview(n.Leading)
	trail := u.layoutPreview(n.Trailing)
	return NewSplit(n.Dir != "v", n.Ratio, 6, lead, trail, nil)
}

type previewHit struct {
	widget.BaseWidget
	child fyne.CanvasObject
	tap   func()
}

func newPreviewHit(child fyne.CanvasObject, tap func()) *previewHit {
	p := &previewHit{child: child, tap: tap}
	p.ExtendBaseWidget(p)
	return p
}

func (p *previewHit) Tapped(*fyne.PointEvent) {
	if p.tap != nil {
		p.tap()
	}
}

func (p *previewHit) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.child)
}

func (u *UI) freshID() string {
	return modelID()
}

func indentRow(depth int, obj fyne.CanvasObject) fyne.CanvasObject {
	if depth <= 0 {
		return obj
	}
	return container.NewBorder(nil, nil, newSpacer(float32(16*depth), 1), nil, obj)
}

type spacer struct {
	widget.BaseWidget
	w, h float32
}

func newSpacer(w, h float32) *spacer {
	s := &spacer{w: w, h: h}
	s.ExtendBaseWidget(s)
	return s
}

func (s *spacer) CreateRenderer() fyne.WidgetRenderer {
	return &spacerRenderer{s: s, rect: canvas.NewRectangle(color.Transparent)}
}

type spacerRenderer struct {
	s    *spacer
	rect *canvas.Rectangle
}

func (r *spacerRenderer) Destroy()            {}
func (r *spacerRenderer) Layout(sz fyne.Size) { r.rect.Resize(sz) }
func (r *spacerRenderer) MinSize() fyne.Size  { return fyne.NewSize(r.s.w, r.s.h) }
func (r *spacerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect}
}
func (r *spacerRenderer) Refresh() {}
