package skin

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
)

// Rect is a fraction of a pane, origin at the top-left.
type Rect struct {
	X, Y, W, H float64
}

// Draw is one vector operation.
type Draw struct {
	Op          string  `json:"op"`
	Rect        []float64 `json:"rect,omitempty"`
	From        []float64 `json:"from,omitempty"`
	To          []float64 `json:"to,omitempty"`
	Points      []float64 `json:"points,omitempty"`
	Fill        string  `json:"fill,omitempty"`
	Stroke      string  `json:"stroke,omitempty"`
	Radius      float64 `json:"radius,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`
}

// Layer is an image placement or a draw list.
type Layer struct {
	Image string  `json:"image,omitempty"`
	Rect  []float64 `json:"rect,omitempty"`
	Slice []float64 `json:"slice,omitempty"`
	Draw  []Draw  `json:"draw,omitempty"`
}

// PaneSkin is one slot.
type PaneSkin struct {
	Layers  []Layer              `json:"layers,omitempty"`
	Content map[string][]float64 `json:"content,omitempty"`
	// Theme restyles the controls inside this slot only. Names it leaves out
	// fall back to the window theme.
	Theme *Theme `json:"theme,omitempty"`
}

// Theme restyles the controls: Fyne color names, size names, fonts, and icons.
type Theme struct {
	// Variant is "dark" or "light". It picks Fyne's built-in palette for
	// color names the skin does not set.
	Variant string `json:"variant,omitempty"`
	// Colors maps a Fyne color name (foreground, button, primary,
	// foregroundOnPrimary, inputBackground, …) to #RRGGBB, #RRGGBBAA, or a
	// palette name.
	Colors map[string]string `json:"colors,omitempty"`
	// Sizes maps a Fyne size name (text, padding, inputRadius, …) to pixels.
	Sizes map[string]float64 `json:"sizes,omitempty"`
	// Fonts maps regular, bold, italic, boldItalic, monospace, or symbol to a
	// .ttf or .otf file, relative to the skin directory.
	Fonts map[string]string `json:"fonts,omitempty"`
	// Icons maps a Fyne icon name (mailSend, search, …) to an .svg or .png
	// file, relative to the skin directory.
	Icons map[string]string `json:"icons,omitempty"`
}

// Pack is a skin.json document.
type Pack struct {
	Name string `json:"name"`
	// Palette names colors so draw operations and theme colors can say
	// "orange" instead of "#FF9900".
	Palette map[string]string   `json:"palette,omitempty"`
	Theme   *Theme              `json:"theme,omitempty"`
	Panes   map[string]PaneSkin `json:"panes"`
}

// Color resolves #RRGGBB, #RRGGBBAA, or a palette name. ok is false for anything else.
func (p Pack) Color(v string) (color.NRGBA, bool) {
	if hex, found := p.Palette[v]; found {
		v = hex
	}
	if IsHex(v) {
		return ParseHex(v), true
	}
	return color.NRGBA{}, false
}

// IsHex reports whether s is #RRGGBB or #RRGGBBAA.
func IsHex(s string) bool {
	if (len(s) != 7 && len(s) != 9) || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// Scale maps a relative rectangle onto a pixel box.
func Scale(r Rect, w, h float64) (x, y, rw, rh float64) {
	return r.X * w, r.Y * h, r.W * w, r.H * h
}

func rectOf(v []float64) Rect {
	if len(v) < 4 {
		return Rect{W: 1, H: 1}
	}
	return Rect{X: v[0], Y: v[1], W: v[2], H: v[3]}
}

// Nine holds source and destination pixel edges for a 9-slice.
type Nine struct {
	SrcL, SrcT, SrcR, SrcB int
	DstL, DstT, DstR, DstB int
	DstW, DstH             int
}

// Slice computes 9-slice edges. Corner sizes come from the source image and stay put when the destination grows.
func Slice(srcW, srcH int, slice []float64, dstW, dstH int) Nine {
	var sl, st, sr, sb float64
	if len(slice) >= 4 {
		sl, st, sr, sb = slice[0], slice[1], slice[2], slice[3]
	}
	n := Nine{DstW: dstW, DstH: dstH}
	n.SrcL = int(sl * float64(srcW))
	n.SrcT = int(st * float64(srcH))
	n.SrcR = int(sr * float64(srcW))
	n.SrcB = int(sb * float64(srcH))
	n.DstL, n.DstT, n.DstR, n.DstB = n.SrcL, n.SrcT, n.SrcR, n.SrcB
	if n.DstL+n.DstR > dstW && dstW > 0 {
		scale := float64(dstW) / float64(n.DstL+n.DstR)
		n.DstL = int(float64(n.DstL) * scale)
		n.DstR = dstW - n.DstL
	}
	if n.DstT+n.DstB > dstH && dstH > 0 {
		scale := float64(dstH) / float64(n.DstT+n.DstB)
		n.DstT = int(float64(n.DstT) * scale)
		n.DstB = dstH - n.DstT
	}
	return n
}

// Loaded is a pack plus the directory it was read from.
type Loaded struct {
	Pack Pack
	Dir  string
}

// Load reads skin.json. A directory of named files and no json still loads as an empty pack.
func Load(dir string) (Loaded, error) {
	b, err := os.ReadFile(filepath.Join(dir, "skin.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Loaded{Dir: dir, Pack: Pack{Name: filepath.Base(dir), Panes: map[string]PaneSkin{}}}, nil
		}
		return Loaded{}, err
	}
	var p Pack
	if err := json.Unmarshal(b, &p); err != nil {
		return Loaded{}, err
	}
	if p.Panes == nil {
		p.Panes = map[string]PaneSkin{}
	}
	return Loaded{Pack: p, Dir: dir}, nil
}

// Slots are the file stems the shell looks for.
func Slots() []string {
	return []string{
		"window", "conversation", "conversation-header", "conversation-transcript", "conversation-composer",
		"select", "select-section", "select-row", "panes", "inspector",
		"settings", "settings-nav", "settings-row", "suite", "suite-tool", "handle",
	}
}

// File returns the SVG path when both SVG and PNG exist, otherwise whichever is present.
func File(dir, slot string) string {
	svg := filepath.Join(dir, slot+".svg")
	png := filepath.Join(dir, slot+".png")
	if _, err := os.Stat(svg); err == nil {
		return svg
	}
	if _, err := os.Stat(png); err == nil {
		return png
	}
	return ""
}

// Plain is the built-in pack of theme-colored rectangles.
func Plain() Pack {
	full := []float64{0, 0, 1, 1}
	body := func() PaneSkin {
		return PaneSkin{
			Layers: []Layer{{Draw: []Draw{{Op: "rect", Rect: full, Fill: "background"}}}},
		}
	}
	chat := PaneSkin{
		Layers: []Layer{{Draw: []Draw{
			{Op: "rect", Rect: full, Fill: "background"},
			{Op: "rect", Rect: []float64{0.03, 0.12, 0.94, 0.70}, Fill: "inputBackground", Radius: 0.01},
		}}},
		Content: map[string][]float64{
			"header":     {0.02, 0.02, 0.96, 0.08},
			"transcript": {0.04, 0.12, 0.92, 0.68},
			"composer":   {0.04, 0.82, 0.92, 0.14},
		},
	}
	return Pack{
		Name: "plain",
		Panes: map[string]PaneSkin{
			"window":       body(),
			"conversation": chat,
			"select":       body(),
			"inspector":    body(),
			"panes":        body(),
			"settings":     body(),
		},
	}
}

// ContentRect returns a content rectangle for a slot key when the pack defines one.
func ContentRect(p Pack, slot, key string, w, h float64) (x, y, rw, rh float64, ok bool) {
	pane, exists := p.Panes[slot]
	if !exists {
		return 0, 0, w, h, false
	}
	raw, exists := pane.Content[key]
	if !exists {
		return 0, 0, w, h, false
	}
	x, y, rw, rh = Scale(rectOf(raw), w, h)
	return x, y, rw, rh, true
}

// ParseHex parses #RRGGBB, or #RRGGBBAA where AA is opacity.
func ParseHex(s string) color.NRGBA {
	var c color.NRGBA
	c.A = 0xff
	if (len(s) == 7 || len(s) == 9) && s[0] == '#' {
		fmtHex(s[1:3], &c.R)
		fmtHex(s[3:5], &c.G)
		fmtHex(s[5:7], &c.B)
		if len(s) == 9 {
			fmtHex(s[7:9], &c.A)
		}
	}
	return c
}

func fmtHex(s string, dst *uint8) {
	var v byte
	for _, r := range s {
		v *= 16
		switch {
		case r >= '0' && r <= '9':
			v += byte(r - '0')
		case r >= 'a' && r <= 'f':
			v += byte(r-'a') + 10
		case r >= 'A' && r <= 'F':
			v += byte(r-'A') + 10
		}
	}
	*dst = v
}

// BlitSlice copies a 9-slice of src into dst.
func BlitSlice(src image.Image, slice []float64, dst *image.NRGBA) {
	sb := src.Bounds()
	n := Slice(sb.Dx(), sb.Dy(), slice, dst.Bounds().Dx(), dst.Bounds().Dy())
	copyRegion(src, dst, 0, 0, n.SrcL, n.SrcT, 0, 0, n.DstL, n.DstT)
	copyRegion(src, dst, sb.Dx()-n.SrcR, 0, n.SrcR, n.SrcT, dst.Bounds().Dx()-n.DstR, 0, n.DstR, n.DstT)
	copyRegion(src, dst, 0, sb.Dy()-n.SrcB, n.SrcL, n.SrcB, 0, dst.Bounds().Dy()-n.DstB, n.DstL, n.DstB)
	copyRegion(src, dst, sb.Dx()-n.SrcR, sb.Dy()-n.SrcB, n.SrcR, n.SrcB, dst.Bounds().Dx()-n.DstR, dst.Bounds().Dy()-n.DstB, n.DstR, n.DstB)
	copyRegion(src, dst, n.SrcL, 0, sb.Dx()-n.SrcL-n.SrcR, n.SrcT, n.DstL, 0, dst.Bounds().Dx()-n.DstL-n.DstR, n.DstT)
	copyRegion(src, dst, n.SrcL, sb.Dy()-n.SrcB, sb.Dx()-n.SrcL-n.SrcR, n.SrcB, n.DstL, dst.Bounds().Dy()-n.DstB, dst.Bounds().Dx()-n.DstL-n.DstR, n.DstB)
	copyRegion(src, dst, 0, n.SrcT, n.SrcL, sb.Dy()-n.SrcT-n.SrcB, 0, n.DstT, n.DstL, dst.Bounds().Dy()-n.DstT-n.DstB)
	copyRegion(src, dst, sb.Dx()-n.SrcR, n.SrcT, n.SrcR, sb.Dy()-n.SrcT-n.SrcB, dst.Bounds().Dx()-n.DstR, n.DstT, n.DstR, dst.Bounds().Dy()-n.DstT-n.DstB)
	copyRegion(src, dst, n.SrcL, n.SrcT, sb.Dx()-n.SrcL-n.SrcR, sb.Dy()-n.SrcT-n.SrcB, n.DstL, n.DstT, dst.Bounds().Dx()-n.DstL-n.DstR, dst.Bounds().Dy()-n.DstT-n.DstB)
}

func copyRegion(src image.Image, dst *image.NRGBA, sx, sy, sw, sh, dx, dy, dw, dh int) {
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return
	}
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			px := sx + x*sw/dw
			py := sy + y*sh/dh
			dst.Set(dx+x, dy+y, src.At(src.Bounds().Min.X+px, src.Bounds().Min.Y+py))
		}
	}
}
