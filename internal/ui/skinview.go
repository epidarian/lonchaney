package ui

import (
	"image"
	"image/color"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/skin"
)

// Frame paints a skin slot behind named content rectangles.
type Frame struct {
	widget.BaseWidget
	Pack     skin.Pack
	Dir      string
	Slot     string
	Parts    map[string]fyne.CanvasObject
	Fallback fyne.CanvasObject
	// Theme, when set, restyles the controls placed in this frame and the
	// theme roles its draw operations name.
	Theme fyne.Theme
}

func NewFrame(pack skin.Pack, slot string, parts map[string]fyne.CanvasObject, fallback fyne.CanvasObject) *Frame {
	f := &Frame{Pack: pack, Slot: slot, Parts: parts, Fallback: fallback}
	f.ExtendBaseWidget(f)
	return f
}

func (f *Frame) CreateRenderer() fyne.WidgetRenderer {
	pane := f.Pack.Panes[f.Slot]
	var items []frameItem
	if named := skin.File(f.Dir, f.Slot); named != "" && !layerNames(pane, named) {
		items = append(items, frameItem{img: newImgLayer(named, []float64{0, 0, 1, 1}, nil)})
	}
	for _, layer := range pane.Layers {
		if layer.Image != "" && f.Dir != "" {
			path := layer.Image
			if !filepath.IsAbs(path) {
				path = filepath.Join(f.Dir, path)
			}
			rect := layer.Rect
			if len(rect) < 4 {
				rect = []float64{0, 0, 1, 1}
			}
			items = append(items, frameItem{img: newImgLayer(path, rect, layer.Slice)})
		}
		for _, d := range layer.Draw {
			switch d.Op {
			case "ellipse":
				c := canvas.NewCircle(f.color(d.Fill))
				c.StrokeColor = f.color(d.Stroke)
				items = append(items, frameItem{back: &placed{obj: c, frac: orFull(d.Rect), radius: d.Radius, stroke: d.StrokeWidth}})
			case "line":
				ln := canvas.NewLine(f.color(d.Stroke))
				items = append(items, frameItem{line: &lineLayer{line: ln, from: d.From, to: d.To, width: d.StrokeWidth}})
			case "polygon":
				sides := uint(len(d.Points) / 2)
				if sides < 3 {
					sides = 3
				}
				poly := canvas.NewPolygon(sides, f.color(d.Fill))
				poly.StrokeColor = f.color(d.Stroke)
				items = append(items, frameItem{poly: &polyLayer{poly: poly, points: d.Points, width: d.StrokeWidth}})
			default:
				rect := canvas.NewRectangle(f.color(d.Fill))
				if d.Stroke != "" {
					rect.StrokeColor = f.color(d.Stroke)
					rect.StrokeWidth = 1
				}
				items = append(items, frameItem{back: &placed{obj: rect, frac: orFull(d.Rect), radius: d.Radius, stroke: d.StrokeWidth}})
			}
		}
	}
	var fronts []placed
	placedAny := false
	for key, obj := range f.Parts {
		frac, ok := pane.Content[key]
		if !ok || obj == nil {
			continue
		}
		fronts = append(fronts, placed{obj: obj, frac: frac, key: key})
		placedAny = true
	}
	if !placedAny && f.Fallback != nil {
		fronts = append(fronts, placed{obj: f.Fallback, frac: []float64{0, 0, 1, 1}})
	}
	if f.Theme != nil {
		for i := range fronts {
			fronts[i].obj = container.NewThemeOverride(fronts[i].obj, f.Theme)
		}
	}
	return &frameRenderer{items: items, fronts: fronts}
}

func layerNames(pane skin.PaneSkin, named string) bool {
	base := filepath.Base(named)
	for _, layer := range pane.Layers {
		if layer.Image != "" && filepath.Base(layer.Image) == base {
			return true
		}
	}
	return false
}

func orFull(rect []float64) []float64 {
	if len(rect) < 4 {
		return []float64{0, 0, 1, 1}
	}
	return rect
}

type placed struct {
	obj    fyne.CanvasObject
	frac   []float64
	key    string
	radius float64
	stroke float64
}

type imgLayer struct {
	path  string
	frac  []float64
	slice []float64
	img   *canvas.Image
	lw    int
	lh    int
}

func newImgLayer(path string, frac, slice []float64) *imgLayer {
	img := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	img.FillMode = canvas.ImageFillStretch
	return &imgLayer{path: path, frac: frac, slice: slice, img: img}
}

func (l *imgLayer) paint(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if l.lw == w && l.lh == h && l.img.Image != nil {
		return
	}
	src, err := skin.RasterFile(l.path, w, h)
	if err != nil {
		return
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	skin.BlitSlice(src, l.slice, dst)
	l.img.Image = dst
	l.img.Refresh()
	l.lw, l.lh = w, h
}

type lineLayer struct {
	line     *canvas.Line
	from, to []float64
	width    float64
}

type polyLayer struct {
	poly   *canvas.Polygon
	points []float64
	width  float64
}

type frameItem struct {
	img  *imgLayer
	back *placed
	line *lineLayer
	poly *polyLayer
}

type frameRenderer struct {
	items  []frameItem
	fronts []placed
}

func (r *frameRenderer) Destroy() {}

func (r *frameRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(r.items)+len(r.fronts))
	for _, it := range r.items {
		switch {
		case it.img != nil:
			out = append(out, it.img.img)
		case it.back != nil:
			out = append(out, it.back.obj)
		case it.line != nil:
			out = append(out, it.line.line)
		case it.poly != nil:
			out = append(out, it.poly.poly)
		}
	}
	for _, o := range r.fronts {
		out = append(out, o.obj)
	}
	return out
}

func (r *frameRenderer) Layout(size fyne.Size) {
	short := size.Width
	if size.Height < short {
		short = size.Height
	}
	for _, it := range r.items {
		switch {
		case it.img != nil:
			x, y, w, h := fracBox(it.img.frac, size)
			it.img.img.Move(fyne.NewPos(x, y))
			it.img.img.Resize(fyne.NewSize(w, h))
			it.img.paint(int(w), int(h))
		case it.back != nil:
			p := it.back
			x, y, w, h := fracBox(p.frac, size)
			p.obj.Move(fyne.NewPos(x, y))
			p.obj.Resize(fyne.NewSize(w, h))
			if rect, ok := p.obj.(*canvas.Rectangle); ok {
				rect.CornerRadius = float32(p.radius) * short
				if p.stroke > 0 {
					rect.StrokeWidth = float32(p.stroke) * short
				}
			}
			if c, ok := p.obj.(*canvas.Circle); ok && p.stroke > 0 {
				c.StrokeWidth = float32(p.stroke) * short
			}
		case it.line != nil:
			ln := it.line
			ln.line.Move(fyne.NewPos(0, 0))
			ln.line.Resize(size)
			ln.line.Position1 = fracPos(ln.from, size)
			ln.line.Position2 = fracPos(ln.to, size)
			if ln.width > 0 {
				ln.line.StrokeWidth = float32(ln.width) * short
			}
		case it.poly != nil:
			p := it.poly
			x, y, w, h := boundsOf(p.points, size)
			p.poly.Move(fyne.NewPos(x, y))
			p.poly.Resize(fyne.NewSize(w, h))
			if p.width > 0 {
				p.poly.StrokeWidth = float32(p.width) * short
			}
			p.poly.Refresh()
		}
	}
	type box struct {
		obj        fyne.CanvasObject
		x, y, w, h float32
		key        string
	}
	boxes := make([]box, len(r.fronts))
	var composer *box
	for i, p := range r.fronts {
		x, y, w, h := fracBox(p.frac, size)
		boxes[i] = box{p.obj, x, y, w, h, p.key}
		if p.key == "composer" {
			composer = &boxes[i]
		}
	}
	if composer != nil {
		if _, ok := composer.obj.(*composerPanel); ok {
			transcript := -1
			for i := range boxes {
				if boxes[i].key == "transcript" {
					transcript = i
				}
			}
			gap := float32(0)
			if transcript >= 0 {
				gap = composer.y - (boxes[transcript].y + boxes[transcript].h)
				if gap < 0 {
					gap = 0
				}
			}
			bottom := composer.y + composer.h
			composer.h = composer.obj.MinSize().Height
			composer.y = bottom - composer.h
			if composer.y < 0 {
				composer.h += composer.y
				composer.y = 0
			}
			if transcript >= 0 {
				next := composer.y - gap
				if next > boxes[transcript].y {
					boxes[transcript].h = next - boxes[transcript].y
				} else {
					boxes[transcript].h = 0
				}
			}
		}
	}
	for _, b := range boxes {
		b.obj.Move(fyne.NewPos(b.x, b.y))
		b.obj.Resize(fyne.NewSize(b.w, b.h))
	}
}

func fracBox(frac []float64, size fyne.Size) (x, y, w, h float32) {
	if len(frac) < 4 {
		return 0, 0, size.Width, size.Height
	}
	return float32(frac[0]) * size.Width, float32(frac[1]) * size.Height, float32(frac[2]) * size.Width, float32(frac[3]) * size.Height
}

func boundsOf(points []float64, size fyne.Size) (x, y, w, h float32) {
	if len(points) < 4 {
		return 0, 0, size.Width, size.Height
	}
	minX, minY := points[0], points[1]
	maxX, maxY := minX, minY
	for i := 0; i+1 < len(points); i += 2 {
		if points[i] < minX {
			minX = points[i]
		}
		if points[i] > maxX {
			maxX = points[i]
		}
		if points[i+1] < minY {
			minY = points[i+1]
		}
		if points[i+1] > maxY {
			maxY = points[i+1]
		}
	}
	x = float32(minX) * size.Width
	y = float32(minY) * size.Height
	w = float32(maxX-minX) * size.Width
	h = float32(maxY-minY) * size.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return x, y, w, h
}

func fracPos(frac []float64, size fyne.Size) fyne.Position {
	if len(frac) < 2 {
		return fyne.NewPos(0, 0)
	}
	return fyne.NewPos(float32(frac[0])*size.Width, float32(frac[1])*size.Height)
}

func (r *frameRenderer) MinSize() fyne.Size { return fyne.NewSize(80, 60) }
func (r *frameRenderer) Refresh()           {}

// color resolves a fill or stroke: empty is transparent; #RRGGBB, #RRGGBBAA,
// and palette names are fixed colors; a Fyne color name follows the frame's
// theme; anything else is the foreground color.
func (f *Frame) color(name string) color.Color {
	if name == "" {
		return color.NRGBA{}
	}
	if c, ok := f.Pack.Color(name); ok {
		return c
	}
	n := fyne.ThemeColorName(name)
	if !themeColorNames[name] {
		n = theme.ColorNameForeground
	}
	if f.Theme != nil {
		return f.Theme.Color(n, fyne.CurrentApp().Settings().ThemeVariant())
	}
	return theme.Color(n)
}
