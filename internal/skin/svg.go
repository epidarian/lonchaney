package skin

import (
	"bytes"
	"image"
	_ "image/png"
	"os"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// RasterFile draws an SVG or PNG into a box of the given pixel size.
func RasterFile(path string, w, h int) (image.Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(path) > 4 && path[len(path)-4:] == ".svg" {
		return RasterSVG(b, w, h)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

// RasterSVG rasterizes SVG bytes into an NRGBA image.
func RasterSVG(data []byte, w, h int) (image.Image, error) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, float64(w), float64(h))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, out, out.Bounds())
	dasher := rasterx.NewDasher(w, h, scanner)
	icon.Draw(dasher, 1)
	return out, nil
}
