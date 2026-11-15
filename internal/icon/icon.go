package icon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	"golang.org/x/image/webp"

	"lonchan/internal/skin"
)

// Decode reads a PNG, JPEG, GIF, WebP, BMP, TIFF, ICO, or SVG icon.
// ext is the suffix to store the file under, including the dot.
func Decode(filename string, data []byte) (image.Image, string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".svg":
		img, err := skin.RasterSVG(data, 256, 256)
		return img, ".svg", err
	case ".webp":
		img, err := webp.Decode(bytes.NewReader(data))
		return img, ext, err
	case ".ico":
		img, err := decodeICO(data)
		return img, ext, err
	}
	if looksSVG(data) {
		img, err := skin.RasterSVG(data, 256, 256)
		return img, ".svg", err
	}
	if isWebP(data) {
		img, err := webp.Decode(bytes.NewReader(data))
		return img, ".webp", err
	}
	if isICO(data) {
		img, err := decodeICO(data)
		return img, ".ico", err
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	if ext == "" {
		ext = "." + format
		if format == "jpeg" {
			ext = ".jpg"
		}
	}
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff":
		return img, ext, nil
	default:
		return img, "." + format, nil
	}
}

func looksSVG(b []byte) bool {
	s := strings.TrimSpace(string(b))
	return strings.HasPrefix(s, "<svg") || strings.HasPrefix(s, "<?xml") || strings.Contains(s[:min(len(s), 200)], "<svg")
}

func isWebP(b []byte) bool {
	return len(b) > 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

func isICO(b []byte) bool {
	if len(b) < 6 {
		return false
	}
	return binary.LittleEndian.Uint16(b[0:2]) == 0 && binary.LittleEndian.Uint16(b[2:4]) == 1 && binary.LittleEndian.Uint16(b[4:6]) > 0
}

func decodeICO(b []byte) (image.Image, error) {
	if !isICO(b) {
		return nil, errors.New("not an ico file")
	}
	count := int(binary.LittleEndian.Uint16(b[4:6]))
	best := -1
	bestArea := 0
	var off, size uint32
	for i := 0; i < count; i++ {
		at := 6 + i*16
		if at+16 > len(b) {
			return nil, errors.New("truncated ico directory")
		}
		w := icoDim(b[at])
		h := icoDim(b[at+1])
		sz := binary.LittleEndian.Uint32(b[at+8 : at+12])
		o := binary.LittleEndian.Uint32(b[at+12 : at+16])
		if w*h >= bestArea {
			best = i
			bestArea = w * h
			off, size = o, sz
		}
	}
	if best < 0 || int(off) < 0 || int(off+size) > len(b) {
		return nil, errors.New("ico image is outside the file")
	}
	chunk := b[off : off+size]
	if len(chunk) >= 8 && string(chunk[:8]) == "\x89PNG\r\n\x1a\n" {
		return png.Decode(bytes.NewReader(chunk))
	}
	return decodeDIB(chunk)
}

func icoDim(b byte) int {
	if b == 0 {
		return 256
	}
	return int(b)
}

func decodeDIB(b []byte) (image.Image, error) {
	if len(b) < 40 {
		return nil, errors.New("truncated dib")
	}
	header := int(binary.LittleEndian.Uint32(b[0:4]))
	if header < 40 || header > len(b) {
		return nil, errors.New("bad dib header")
	}
	w := int(int32(binary.LittleEndian.Uint32(b[4:8])))
	h := int(int32(binary.LittleEndian.Uint32(b[8:12])))
	if h < 0 {
		h = -h
	} else if h > 1 {
		h = h / 2
	}
	bpp := int(binary.LittleEndian.Uint16(b[14:16]))
	if w < 1 || h < 1 || (bpp != 24 && bpp != 32) {
		return nil, errors.New("unsupported ico bitmap")
	}
	pix := b[header:]
	stride := ((w*bpp + 31) / 32) * 4
	need := stride * h
	if len(pix) < need {
		return nil, errors.New("truncated ico bitmap")
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := pix[(h-1-y)*stride : (h-1-y)*stride+stride]
		for x := 0; x < w; x++ {
			i := x * bpp / 8
			a := byte(255)
			if bpp == 32 {
				a = row[i+3]
			}
			out.SetNRGBA(x, y, color.NRGBA{R: row[i+2], G: row[i+1], B: row[i], A: a})
		}
	}
	return out, nil
}
