package ui

import (
	"image/color"
	"testing"
)

func TestLetterDiscCentersInk(t *testing.T) {
	for _, name := range []string{"Jason", "alice", "Quentin", "george"} {
		const px = 64
		img := letterDisc(name, px)
		minX, minY, maxX, maxY := px, px, 0, 0
		n := 0
		for y := 0; y < px; y++ {
			for x := 0; x < px; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r > 50000 && g > 50000 && b > 50000 {
					n++
					if x < minX {
						minX = x
					}
					if y < minY {
						minY = y
					}
					if x > maxX {
						maxX = x
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
		if n == 0 {
			t.Fatalf("%s drew no letter", name)
		}
		cx := float64(minX+maxX) / 2
		cy := float64(minY+maxY) / 2
		mid := float64(px-1) / 2
		if cx < mid-1.5 || cx > mid+1.5 || cy < mid-1.5 || cy > mid+1.5 {
			t.Fatalf("%s ink center %.1f,%.1f want %.1f", name, cx, cy, mid)
		}
		corner := img.At(0, 0).(color.NRGBA)
		if corner.A != 0 {
			t.Fatalf("%s painted the corner", name)
		}
	}
}
