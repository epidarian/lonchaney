// Command lcarsgen writes skins/lcars/skin.json, an LCARS-style skin.
// Shapes are laid out in pixels on a reference frame size, then converted to
// the fractions skin.json uses. Run from the repo root:
//
//	go run ./tools/lcarsgen skins/lcars/skin.json
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// Draw operations name palette entries; skin.json maps each name to its hex.
const (
	black    = "black"
	orange   = "orange"
	tan      = "tan"
	peach    = "peach"
	lilac    = "lilac"
	lavender = "lavender"
	blue     = "blue"
	sky      = "sky"
	rust     = "rust"
	gold     = "gold"
)

var palette = [][2]string{
	{black, "#000000"}, {orange, "#FF9900"}, {tan, "#FFCC99"}, {peach, "#FF9966"},
	{lilac, "#CC99CC"}, {lavender, "#9999CC"}, {blue, "#9999FF"}, {sky, "#99CCFF"},
	{rust, "#CC6666"}, {gold, "#FFCC66"},
	{"panel", "#14122A"}, {"panelHigh", "#22203A"}, {"violet", "#3D2E5C"},
	{"violetHigh", "#4A3A70"}, {"dim", "#8A86B8"}, {"line", "#3A3355"},
}

// theme is a skin.json theme block.
type theme struct {
	Variant string             `json:"variant,omitempty"`
	Colors  map[string]string  `json:"colors,omitempty"`
	Sizes   map[string]float64 `json:"sizes,omitempty"`
}

// windowTheme restyles every control: tan text on black, orange primary
// buttons with black text, violet standard buttons, and pill-shaped inputs.
var windowTheme = theme{
	Variant: "dark",
	Colors: map[string]string{
		"background":          black,
		"foreground":          tan,
		"primary":             orange,
		"foregroundOnPrimary": black,
		"button":              "violet",
		"pressed":             "violetHigh",
		"hover":               "#2E2547",
		"disabledButton":      "#1E1A2E",
		"disabled":            "#6E6A98",
		"placeholder":         "dim",
		"selection":           "#5A3A6E",
		"focus":               "#FF9900AA",
		"inputBackground":     "panel",
		"inputBorder":         "line",
		"separator":           "line",
		"scrollBar":           "#9999CC88",
		"menuBackground":      "panel",
		"overlayBackground":   "panel",
		"headerBackground":    "panelHigh",
		"hyperlink":           sky,
		"error":               "#FF5555",
		"success":             "#99CC99",
		"warning":             gold,
		"shadow":              "#00000088",
	},
	Sizes: map[string]float64{
		"inputRadius":     16,
		"selectionRadius": 8,
	},
}

// Slot accents: the Selection pane leans orange, the Inspector leans lavender.
var slotThemes = map[string]*theme{
	"select": {Colors: map[string]string{
		"primary":   peach,
		"selection": "#5A3A1E",
		"hover":     "#33240F",
	}},
	"inspector": {Colors: map[string]string{
		"foreground": "#CCCCFF",
		"disabled":   lavender,
		"primary":    lilac,
	}},
	"settings-nav": {Colors: map[string]string{
		"selection": "#5A3A1E",
		"hover":     "#33240F",
	}},
}

type op struct {
	Op     string    `json:"op"`
	Rect   []float64 `json:"rect"`
	Fill   string    `json:"fill"`
	Radius float64   `json:"radius,omitempty"`
}

// frame collects ops in reference pixels.
type frame struct {
	w, h float64
	px   []pxRect
}

type pxRect struct {
	x, y, w, h, r float64
	fill          string
}

func (f *frame) rect(x, y, w, h float64, fill string) {
	f.px = append(f.px, pxRect{x, y, w, h, 0, fill})
}
func (f *frame) round(x, y, w, h, r float64, fill string) {
	f.px = append(f.px, pxRect{x, y, w, h, r, fill})
}

// elbowOuter is the rounded block whose top-left corner is an elbow's outer curve.
func (f *frame) elbowOuter(x, y, side, outer float64, fill string) {
	f.round(x, y, side+outer, outer*2, outer, fill)
}

// elbowInner is the black block whose top-left corner cuts the elbow's inner curve.
// Draw it after the bar and the sidebar.
func (f *frame) elbowInner(x, y, side, bar, outer, inner float64) {
	f.round(x+side, y+bar, outer+inner*3, outer*2, inner, black)
}

// flip mirrors ops drawn from index from onward.
func (f *frame) flip(from int, x, y bool) {
	for i := from; i < len(f.px); i++ {
		p := &f.px[i]
		if x {
			p.x = f.w - p.x - p.w
		}
		if y {
			p.y = f.h - p.y - p.h
		}
	}
}

func r4(v float64) float64 { return math.Round(v*10000) / 10000 }

func (f *frame) ops() []op {
	short := math.Min(f.w, f.h)
	out := make([]op, 0, len(f.px))
	for _, p := range f.px {
		out = append(out, op{
			Op:     "rect",
			Rect:   []float64{r4(p.x / f.w), r4(p.y / f.h), r4(p.w / f.w), r4(p.h / f.h)},
			Fill:   p.fill,
			Radius: r4(p.r / short),
		})
	}
	return out
}

func (f *frame) box(x, y, w, h float64) []float64 {
	return []float64{r4(x / f.w), r4(y / f.h), r4(w / f.w), r4(h / f.h)}
}

// bars lays colored segments left to right with a fixed gap; the last one is a pill.
func (f *frame) bars(x, y, h, end, gap float64, segs []float64, colors []string) {
	for i, w := range segs {
		last := i == len(segs)-1
		if last {
			w = end - x
		}
		if last {
			f.round(x, y, w, h, h/2, colors[i])
		} else {
			f.rect(x, y, w, h, colors[i])
		}
		x += w + gap
	}
}

// column stacks sidebar blocks top to bottom with a fixed gap.
func (f *frame) column(x, y, w, gap float64, heights []float64, colors []string) {
	for i, h := range heights {
		f.rect(x, y, w, h, colors[i])
		y += h + gap
	}
}

func conversation() (*frame, map[string][]float64) {
	f := &frame{w: 960, h: 780}
	const side, bar, outer, inner, gap = 100.0, 18.0, 30.0, 14.0, 6.0
	f.rect(0, 0, f.w, f.h, black)

	// Upper section: sidebar coming down, elbow into a bar under the header.
	start := len(f.px)
	f.elbowOuter(0, 0, side, outer, orange)
	f.bars(side, 0, bar, f.w, gap, []float64{457, 140, 160, 0}, []string{orange, lilac, tan, peach})
	f.column(0, outer, side, 0, []float64{50}, []string{orange})
	f.elbowInner(0, 0, side, bar, outer, inner)
	// Mirror vertically inside the 80px band so the elbow turns at the bottom.
	for i := start; i < len(f.px); i++ {
		p := &f.px[i]
		p.y = 80 - p.y - p.h
	}

	// Lower section: elbow into a bar at the top, segmented sidebar, elbow into a bar at the bottom.
	const top = 92.0
	f.elbowOuter(0, top, side, outer, orange)
	f.bars(side, top, bar, f.w, gap, []float64{300, 60, 380, 0}, []string{orange, rust, lavender, sky})
	f.column(0, top+outer, side, gap, []float64{128, 124, 84, 120, 148}, []string{orange, peach, lavender, lilac, blue})
	f.elbowInner(0, top, side, bar, outer, inner)

	start = len(f.px)
	f.elbowOuter(0, 0, side, outer, blue)
	f.bars(side, 0, bar, f.w, gap, []float64{260, 90, 400, 0}, []string{blue, gold, lilac, tan})
	f.elbowInner(0, 0, side, bar, outer, inner)
	f.flip(start, false, true)

	// Thin divider between the transcript and the composer.
	f.round(116, 605, 560, 4, 2, lavender)
	f.round(682, 605, 266, 4, 2, lilac)

	content := map[string][]float64{
		"header":     f.box(116, 8, 832, 50),
		"transcript": f.box(116, 122, 832, 474),
		"composer":   f.box(116, 620, 832, 132),
	}
	return f, content
}

// rail is a narrow pane: a thin sidebar down one edge and an elbow into a bottom bar.
func rail(w float64, mirror bool) *frame {
	f := &frame{w: w, h: 780}
	const side, bar, outer, inner, gap = 6.0, 14.0, 16.0, 6.0, 6.0
	f.rect(0, 0, f.w, f.h, black)
	start := len(f.px)
	f.column(0, 0, side, gap, []float64{60, 184, 214, 224}, []string{orange, peach, lavender, lilac})
	f.rect(0, 706, side, 58, blue)
	// Bottom elbow: build it as a top-left elbow, then flip it down.
	e := len(f.px)
	f.elbowOuter(0, 0, side, outer, blue)
	f.bars(side, 0, bar, f.w, gap, []float64{0.56 * w, 0.14 * w, 0}, []string{blue, gold, tan})
	f.elbowInner(0, 0, side, bar, outer, inner)
	f.flip(e, false, true)
	if mirror {
		f.flip(start, true, false)
	}
	return f
}

// settingsNav has a thin rail down its right edge that turns into a bar along the top.
func settingsNav() *frame {
	f := &frame{w: 280, h: 780}
	const side, bar, outer, inner, gap = 6.0, 10.0, 16.0, 6.0, 6.0
	f.rect(0, 0, f.w, f.h, black)
	start := len(f.px)
	f.elbowOuter(0, 0, side, outer, orange)
	f.bars(side, 0, bar, 130, gap, []float64{74, 18, 0}, []string{orange, rust, tan})
	f.column(0, outer, side, gap, []float64{168, 124, 184}, []string{orange, lilac, lavender})
	f.round(0, outer+168+124+184+gap*3, side, 168, 3, peach)
	f.elbowInner(0, 0, side, bar, outer, inner)
	f.flip(start, true, false)
	return f
}

// settingsBody puts a short elbow in the top-right corner, clear of the page title.
func settingsBody() *frame {
	f := &frame{w: 920, h: 780}
	const side, bar, outer, inner, gap, y = 8.0, 16.0, 24.0, 10.0, 6.0, 8.0
	f.rect(0, 0, f.w, f.h, black)
	start := len(f.px)
	f.elbowOuter(0, y, side, outer, orange)
	f.bars(side, y, bar, 420, gap, []float64{300, 60, 0}, []string{orange, lilac, tan})
	f.column(0, y+outer, side, gap, []float64{140, 60}, []string{orange, lavender})
	f.round(0, y+outer+140+60+gap*2, side, 40, 4, peach)
	f.elbowInner(0, y, side, bar, outer, inner)
	f.flip(start, true, false)
	return f
}

func flat(fill string) *frame {
	f := &frame{w: 100, h: 100}
	f.rect(0, 0, 100, 100, fill)
	return f
}

type pane struct {
	name    string
	f       *frame
	content map[string][]float64
}

// indent re-indents a JSON block for nesting at depth spaces.
func indent(v any, depth int) string {
	b, _ := json.MarshalIndent(v, strings.Repeat(" ", depth), "  ")
	return string(b)
}

func main() {
	conv, content := conversation()
	panes := []pane{
		{"window", flat(black), nil},
		{"handle", flat(black), nil},
		{"conversation", conv, content},
		{"select", rail(160, false), nil},
		{"inspector", rail(150, true), nil},
		{"settings-nav", settingsNav(), nil},
		{"settings", settingsBody(), nil},
	}
	var b strings.Builder
	b.WriteString("{\n  \"name\": \"LCARS\",\n  \"palette\": {\n")
	for i, kv := range palette {
		comma := ","
		if i == len(palette)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "    %q: %q%s\n", kv[0], kv[1], comma)
	}
	fmt.Fprintf(&b, "  },\n  \"theme\": %s,\n  \"panes\": {\n", indent(windowTheme, 2))
	for i, p := range panes {
		fmt.Fprintf(&b, "    %q: {\n", p.name)
		if t, ok := slotThemes[p.name]; ok {
			fmt.Fprintf(&b, "      \"theme\": %s,\n", indent(t, 6))
		}
		b.WriteString("      \"layers\": [\n        {\n          \"draw\": [\n")
		ops := p.f.ops()
		for j, o := range ops {
			line, _ := json.Marshal(o)
			comma := ","
			if j == len(ops)-1 {
				comma = ""
			}
			fmt.Fprintf(&b, "            %s%s\n", line, comma)
		}
		b.WriteString("          ]\n        }\n      ]")
		if p.content != nil {
			b.WriteString(",\n      \"content\": {\n")
			keys := []string{"header", "transcript", "composer"}
			for j, k := range keys {
				line, _ := json.Marshal(p.content[k])
				comma := ","
				if j == len(keys)-1 {
					comma = ""
				}
				fmt.Fprintf(&b, "        %q: %s%s\n", k, line, comma)
			}
			b.WriteString("      }")
		}
		b.WriteString("\n    }")
		if i < len(panes)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  }\n}\n")
	if err := os.WriteFile(os.Args[1], []byte(b.String()), 0o644); err != nil {
		panic(err)
	}
}
