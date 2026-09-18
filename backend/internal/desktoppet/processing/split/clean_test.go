package split

import (
	"image"
	"testing"
)

func TestCleanCellContentRemovesBleed(t *testing.T) {
	cell := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for i := range cell.Pix {
		cell.Pix[i] = 255
	}
	for y := 30; y < 90; y++ {
		for x := 20; x < 80; x++ {
			i := cell.PixOffset(x, y)
			cell.Pix[i], cell.Pix[i+1], cell.Pix[i+2] = 20, 20, 20
		}
	}
	for y := 0; y < 2; y++ {
		for x := 10; x < 90; x++ {
			i := cell.PixOffset(x, y)
			cell.Pix[i], cell.Pix[i+1], cell.Pix[i+2] = 30, 30, 30
		}
	}

	cleanCellContent(cell)

	for y := 0; y < 10; y++ {
		for x := 10; x < 90; x++ {
			i := cell.PixOffset(x, y)
			if cell.Pix[i] != 255 {
				t.Fatalf("bleed not cleared at (%d,%d): %d", x, y, cell.Pix[i])
			}
		}
	}
	i := cell.PixOffset(50, 60)
	if cell.Pix[i] == 255 {
		t.Fatal("main subject was cleared")
	}
}

func TestCleanCellContentKeepsFullFrame(t *testing.T) {
	cell := image.NewNRGBA(image.Rect(0, 0, 50, 50))
	for i := range cell.Pix {
		cell.Pix[i] = 255
	}
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			i := cell.PixOffset(x, y)
			cell.Pix[i], cell.Pix[i+1], cell.Pix[i+2] = 40, 40, 40
		}
	}
	cleanCellContent(cell)
	i := cell.PixOffset(25, 25)
	if cell.Pix[i] != 40 {
		t.Fatal("full-frame content should be untouched")
	}
}
