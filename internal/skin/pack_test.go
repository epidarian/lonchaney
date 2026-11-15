package skin

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestScale(t *testing.T) {
	x, y, w, h := Scale(Rect{X: 0.04, Y: 0.12, W: 0.92, H: 0.68}, 100, 200)
	if math.Abs(x-4) > 1e-9 || math.Abs(y-24) > 1e-9 || math.Abs(w-92) > 1e-9 || math.Abs(h-136) > 1e-9 {
		t.Fatalf("%v %v %v %v", x, y, w, h)
	}
}

func TestNineSliceCornersStay(t *testing.T) {
	a := Slice(100, 100, []float64{0.1, 0.1, 0.1, 0.1}, 100, 100)
	b := Slice(100, 100, []float64{0.1, 0.1, 0.1, 0.1}, 200, 100)
	if a.DstL != 10 || b.DstL != 10 || a.DstL != b.DstL || a.DstR != b.DstR {
		t.Fatalf("%+v %+v", a, b)
	}
}

func TestBlit(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 20), A: 255})
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	BlitSlice(src, []float64{0.2, 0.2, 0.2, 0.2}, dst)
	if dst.NRGBAAt(0, 0).A == 0 {
		t.Fatal("empty corner")
	}
}

func TestNamedAndJSON(t *testing.T) {
	p := Plain()
	_, _, _, _, ok := ContentRect(p, "conversation", "transcript", 100, 100)
	if !ok {
		t.Fatal("content")
	}
	if _, _, _, _, ok := ContentRect(p, "select", "transcript", 10, 10); ok {
		t.Fatal("select has no transcript rect")
	}
}

func TestPaletteAndAlpha(t *testing.T) {
	p := Pack{Palette: map[string]string{"orange": "#FF9900"}}
	if c, ok := p.Color("orange"); !ok || c != (color.NRGBA{0xFF, 0x99, 0x00, 0xFF}) {
		t.Fatalf("palette %v %v", c, ok)
	}
	if c, ok := p.Color("#11223380"); !ok || c != (color.NRGBA{0x11, 0x22, 0x33, 0x80}) {
		t.Fatalf("alpha %v %v", c, ok)
	}
	for _, bad := range []string{"", "foreground", "#12345", "#GGGGGG"} {
		if _, ok := p.Color(bad); ok {
			t.Fatalf("%q resolved", bad)
		}
	}
}
