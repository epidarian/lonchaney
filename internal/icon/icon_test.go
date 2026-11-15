package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"
)

func TestDecodePNGSVGICO(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	pngBytes := buf.Bytes()
	if _, ext, err := Decode("a.png", pngBytes); err != nil || ext != ".png" {
		t.Fatalf("png %s %v", ext, err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" fill="red"/></svg>`)
	if _, ext, err := Decode("a.svg", svg); err != nil || ext != ".svg" {
		t.Fatalf("svg %s %v", ext, err)
	}
	ico := wrapPNGICO(pngBytes)
	img, ext, err := Decode("a.ico", ico)
	if err != nil || ext != ".ico" || img.Bounds().Dx() != 2 {
		t.Fatalf("ico %s %v %v", ext, err, img)
	}
}

func wrapPNGICO(pngBytes []byte) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	buf.Write([]byte{2, 2, 0, 0})
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(pngBytes)))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(22))
	buf.Write(pngBytes)
	return buf.Bytes()
}
