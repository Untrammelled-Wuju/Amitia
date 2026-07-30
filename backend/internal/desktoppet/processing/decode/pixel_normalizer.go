package decode

import (
	"image"
	"image/color"
)

type AlphaStats struct {
	TotalPixels           int
	TransparentPixels     int
	SemiTransparentPixels int
	OpaquePixels          int
	AlphaVariance         float64
	HasValidAlpha         bool
	AllOpaque             bool
	AllTransparent        bool
}

func AnalyzeAlpha(img *image.NRGBA) AlphaStats {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	total := w * h

	var transparent, semi, opaque int
	var sumAlpha, sumAlphaSq float64

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			a := img.NRGBAAt(x, y).A
			af := float64(a)
			sumAlpha += af
			sumAlphaSq += af * af
			switch {
			case a == 0:
				transparent++
			case a == 255:
				opaque++
			default:
				semi++
			}
		}
	}

	stats := AlphaStats{
		TotalPixels:           total,
		TransparentPixels:     transparent,
		SemiTransparentPixels: semi,
		OpaquePixels:          opaque,
	}

	if total > 0 {
		mean := sumAlpha / float64(total)
		stats.AlphaVariance = sumAlphaSq/float64(total) - mean*mean
		stats.HasValidAlpha = float64(transparent) > float64(total)*0.01
		stats.AllOpaque = opaque == total
		stats.AllTransparent = transparent == total
	}

	return stats
}

func UnpremultiplyRGBA(img *image.RGBA) *image.NRGBA {
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.RGBAAt(x, y)
			dx := x - bounds.Min.X
			dy := y - bounds.Min.Y
			if c.A == 0 {
				dst.SetNRGBA(dx, dy, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				continue
			}
			a := uint32(c.A)
			r := (uint32(c.R)*255 + a/2) / a
			g := (uint32(c.G)*255 + a/2) / a
			b := (uint32(c.B)*255 + a/2) / a
			if r > 255 {
				r = 255
			}
			if g > 255 {
				g = 255
			}
			if b > 255 {
				b = 255
			}
			dst.SetNRGBA(dx, dy, color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: c.A})
		}
	}

	return dst
}

func CleanTransparentRGB(img *image.NRGBA) *image.NRGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetNRGBA(x, y, img.NRGBAAt(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	maxRadius := 3
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := dst.NRGBAAt(x, y)
			if c.A > 0 {
				continue
			}
			if nearest, ok := findNearestOpaque(dst, x, y, w, h, maxRadius); ok {
				dst.SetNRGBA(x, y, color.NRGBA{
					R: nearest.R,
					G: nearest.G,
					B: nearest.B,
					A: 0,
				})
			}
		}
	}

	return dst
}

func findNearestOpaque(img *image.NRGBA, cx, cy, w, h, maxRadius int) (color.NRGBA, bool) {
	for r := 1; r <= maxRadius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if abs(dx) < r && abs(dy) < r {
					continue
				}
				nx := cx + dx
				ny := cy + dy
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				c := img.NRGBAAt(nx, ny)
				if c.A > 0 {
					return c, true
				}
			}
		}
	}
	return color.NRGBA{}, false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}