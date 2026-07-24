// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image"
	"image/color"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

type CanvasConfig struct {
	OutputWidth                int
	OutputHeight               int
	TargetCharacterHeightRatio float64
}

func DefaultCanvasConfig() CanvasConfig {
	return CanvasConfig{
		OutputWidth:                512,
		OutputHeight:               512,
		TargetCharacterHeightRatio: 0.8,
	}
}

func ComputeSequenceScale(maxBox backgroundremoval.SubjectBox, cfg CanvasConfig) float64 {
	if maxBox.Empty || maxBox.Height <= 0 {
		return 1.0
	}
	if cfg.OutputHeight <= 0 || cfg.TargetCharacterHeightRatio <= 0 {
		return 1.0
	}
	targetHeight := float64(cfg.OutputHeight) * cfg.TargetCharacterHeightRatio
	return targetHeight / float64(maxBox.Height)
}

func ComputeActionScaleBaseline(defaultIdleMaxBox backgroundremoval.SubjectBox, cfg CanvasConfig) float64 {
	return ComputeSequenceScale(defaultIdleMaxBox, cfg)
}

func ApplyScale(img image.Image, scale float64) image.Image {
	if img == nil || scale <= 0 {
		return img
	}
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return img
	}

	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)
	if newW <= 0 || newH <= 0 {
		return img
	}

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	scaleX := float64(srcW) / float64(newW)
	scaleY := float64(srcH) / float64(newH)

	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + int(float64(y)*scaleY)
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + int(float64(x)*scaleX)
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			c := img.At(srcX, srcY)
			r, g, b, a := c.RGBA()
			dst.Set(x, y, color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}

func NormalizeCanvas(img image.Image, scale float64, anchorX, anchorY float64, cfg CanvasConfig) image.Image {
	if img == nil {
		return nil
	}
	if cfg.OutputWidth <= 0 || cfg.OutputHeight <= 0 {
		return img
	}

	scaled := ApplyScale(img, scale)
	sBounds := scaled.Bounds()
	sW := sBounds.Dx()
	sH := sBounds.Dy()

	canvas := image.NewNRGBA(image.Rect(0, 0, cfg.OutputWidth, cfg.OutputHeight))

	centerX := anchorX * float64(cfg.OutputWidth)
	centerY := anchorY * float64(cfg.OutputHeight)
	dstX0 := int(centerX - float64(sW)/2.0)
	dstY0 := int(centerY - float64(sH)/2.0)

	for y := 0; y < sH; y++ {
		dstY := dstY0 + y
		if dstY < 0 || dstY >= cfg.OutputHeight {
			continue
		}
		srcY := sBounds.Min.Y + y
		for x := 0; x < sW; x++ {
			dstX := dstX0 + x
			if dstX < 0 || dstX >= cfg.OutputWidth {
				continue
			}
			srcX := sBounds.Min.X + x
			c := scaled.At(srcX, srcY)
			canvas.Set(dstX, dstY, c)
		}
	}
	return canvas
}
