package local

import (
	"image"
	"image/color"
	"testing"
)

func fillRect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func TestRemoveByColorConnectedRescuesDetachedDarkSubject(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	fillRect(img, 0, 0, 100, 100, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	fillRect(img, 30, 30, 60, 60, color.NRGBA{R: 20, G: 20, B: 20, A: 255})

	result := removeByColorConnectedNRGBA(img, 255, 255, 255, 30)

	if got := result.NRGBAAt(50, 50); got.A != 255 {
		t.Fatalf("dark subject should stay opaque, got alpha %d", got.A)
	}
	if got := result.NRGBAAt(5, 5); got.A != 0 {
		t.Fatalf("border background should be cleared, got alpha %d", got.A)
	}
}

func TestTrimMaskEdgeBandsRemovesEdgeBarKeepsSubject(t *testing.T) {
	mask := make([][]bool, 100)
	for y := 0; y < 100; y++ {
		mask[y] = make([]bool, 100)
		for x := 30; x < 60; x++ {
			mask[y][x] = true
		}
		for x := 96; x < 100; x++ {
			mask[y][x] = true
		}
	}

	trimMaskEdgeBands(mask)

	if !mask[50][45] {
		t.Fatalf("subject body should be kept")
	}
	for y := 0; y < 100; y++ {
		for x := 96; x < 100; x++ {
			if mask[y][x] {
				t.Fatalf("edge bar at (%d,%d) should be cleared", x, y)
			}
		}
	}
}

func TestTrimMaskEdgeBandsKeepsEverythingWhenSingleComponent(t *testing.T) {
	mask := make([][]bool, 100)
	for y := 0; y < 100; y++ {
		mask[y] = make([]bool, 100)
		for x := 0; x < 100; x++ {
			mask[y][x] = true
		}
	}

	trimMaskEdgeBands(mask)

	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if !mask[y][x] {
				t.Fatalf("single full component must not be trimmed at (%d,%d)", x, y)
			}
		}
	}
}

func TestRemoveBorderNearWhiteRegionsClearsWhiteBoxKeepsDarkSubject(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	fillRect(img, 0, 0, 100, 100, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
	fillRect(img, 35, 35, 65, 65, color.NRGBA{R: 30, G: 30, B: 40, A: 255})

	mask := make([][]bool, 100)
	for y := 0; y < 100; y++ {
		mask[y] = make([]bool, 100)
		for x := 0; x < 100; x++ {
			mask[y][x] = true
		}
	}

	removeBorderNearWhiteRegions(mask, img, img.Bounds())

	if !mask[50][50] {
		t.Fatalf("dark subject inside white box should be kept")
	}
	if mask[10][10] {
		t.Fatalf("white box area should be cleared")
	}
}
