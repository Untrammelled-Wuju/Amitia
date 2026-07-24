// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image/color"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func TestCanvasComputeSequenceScale(t *testing.T) {
	cfg := DefaultCanvasConfig()
	box := backgroundremoval.SubjectBox{MinX: 0, MinY: 0, MaxX: 99, MaxY: 99, Width: 100, Height: 100, Empty: false}
	scale := ComputeSequenceScale(box, cfg)
	expected := (512.0 * 0.8) / 100.0
	if scale != expected {
		t.Errorf("expected scale %v, got %v", expected, scale)
	}
}

func TestCanvasComputeSequenceScaleEmpty(t *testing.T) {
	cfg := DefaultCanvasConfig()
	box := backgroundremoval.SubjectBox{Empty: true}
	scale := ComputeSequenceScale(box, cfg)
	if scale != 1.0 {
		t.Errorf("expected scale 1.0 for empty box, got %v", scale)
	}
}

func TestCanvasComputeSequenceScaleZeroHeight(t *testing.T) {
	cfg := DefaultCanvasConfig()
	box := backgroundremoval.SubjectBox{MinX: 0, MinY: 0, MaxX: 0, MaxY: 0, Width: 1, Height: 0, Empty: false}
	scale := ComputeSequenceScale(box, cfg)
	if scale != 1.0 {
		t.Errorf("expected scale 1.0 for zero height, got %v", scale)
	}
}

func TestCanvasComputeActionScaleBaseline(t *testing.T) {
	cfg := DefaultCanvasConfig()
	box := backgroundremoval.SubjectBox{MinX: 0, MinY: 0, MaxX: 99, MaxY: 99, Width: 100, Height: 100, Empty: false}
	scale := ComputeActionScaleBaseline(box, cfg)
	expected := (512.0 * 0.8) / 100.0
	if scale != expected {
		t.Errorf("expected baseline %v, got %v", expected, scale)
	}
}

func TestCanvasApplyScaleUp(t *testing.T) {
	img := newTestImage(100, 50, color.NRGBA{255, 0, 0, 255})
	scaled := ApplyScale(img, 2.0)
	b := scaled.Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Errorf("expected 200x100, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestCanvasApplyScaleDown(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 255, 0, 255})
	scaled := ApplyScale(img, 0.5)
	b := scaled.Bounds()
	if b.Dx() != 50 || b.Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", b.Dx(), b.Dy())
	}
	c := scaled.At(25, 25)
	_, g, _, _ := c.RGBA()
	if g == 0 {
		t.Errorf("expected green pixel, got %v", c)
	}
}

func TestCanvasApplyScaleAlpha(t *testing.T) {
	img := newTestImage(40, 40, color.NRGBA{10, 20, 30, 200})
	scaled := ApplyScale(img, 2.0)
	c := scaled.At(20, 20)
	_, _, _, a := c.RGBA()
	expected := uint32(200) * 257
	if a != expected {
		t.Errorf("expected alpha %d, got %d", expected, a)
	}
}

func TestCanvasApplyScaleZero(t *testing.T) {
	img := newTestImage(10, 10, color.NRGBA{255, 255, 255, 255})
	out := ApplyScale(img, 0)
	if out != img {
		t.Errorf("expected same image for zero scale")
	}
}

func TestCanvasNormalizeCanvasSize(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 255, 255})
	cfg := DefaultCanvasConfig()
	out := NormalizeCanvas(img, 1.0, 0.5, 0.5, cfg)
	b := out.Bounds()
	if b.Dx() != cfg.OutputWidth || b.Dy() != cfg.OutputHeight {
		t.Errorf("expected %dx%d, got %dx%d", cfg.OutputWidth, cfg.OutputHeight, b.Dx(), b.Dy())
	}
}

func TestCanvasNormalizeCanvasCenter(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 255, 255})
	cfg := DefaultCanvasConfig()
	out := NormalizeCanvas(img, 1.0, 0.5, 0.5, cfg)
	c := out.At(256, 256)
	_, _, _, a := c.RGBA()
	if a == 0 {
		t.Errorf("expected non-transparent pixel at center, got %v", c)
	}
}

func TestCanvasNormalizeCanvasAnchorOffset(t *testing.T) {
	img := newTestImage(40, 40, color.NRGBA{255, 255, 0, 255})
	cfg := DefaultCanvasConfig()
	out := NormalizeCanvas(img, 1.0, 0.5, 0.5, cfg)
	dstX0 := 256 - 20
	dstY0 := 256 - 20
	c := out.At(dstX0, dstY0)
	_, _, _, a := c.RGBA()
	if a == 0 {
		t.Errorf("expected non-transparent at image top-left (%d,%d), got %v", dstX0, dstY0, c)
	}
}

func TestCanvasNormalizeCanvasNoCrop(t *testing.T) {
	img := newTestImage(200, 200, color.NRGBA{255, 0, 0, 255})
	cfg := DefaultCanvasConfig()
	out := NormalizeCanvas(img, 1.0, 0.5, 0.5, cfg)
	dstX0 := 256 - 100
	dstY0 := 256 - 100
	c := out.At(dstX0, dstY0)
	_, _, _, a := c.RGBA()
	if a == 0 {
		t.Errorf("expected non-transparent at image top-left, got %v", c)
	}
	c2 := out.At(dstX0+199, dstY0+199)
	_, _, _, a2 := c2.RGBA()
	if a2 == 0 {
		t.Errorf("expected non-transparent at image bottom-right, got %v", c2)
	}
}
