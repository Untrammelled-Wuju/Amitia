// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image"
	"image/color"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func TestAlignApplyTranslation(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 40, 40, 60, 60, color.NRGBA{255, 255, 255, 255})
	cfg := DefaultCanvasConfig()
	out := applyTranslation(img, 10, 20, cfg)
	b := out.Bounds()
	if b.Dx() != cfg.OutputWidth || b.Dy() != cfg.OutputHeight {
		t.Errorf("expected canvas size %dx%d, got %dx%d", cfg.OutputWidth, cfg.OutputHeight, b.Dx(), b.Dy())
	}
	c := out.At(50, 60)
	_, _, _, a := c.RGBA()
	if a == 0 {
		t.Errorf("expected non-transparent at (50,60), got %v", c)
	}
	c2 := out.At(40, 40)
	_, _, _, a2 := c2.RGBA()
	if a2 != 0 {
		t.Errorf("expected transparent at original (40,40), got %v", c2)
	}
}

func TestAlignApplyTranslationNegative(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 40, 40, 60, 60, color.NRGBA{255, 255, 255, 255})
	cfg := DefaultCanvasConfig()
	out := applyTranslation(img, -5, -5, cfg)
	c := out.At(35, 35)
	_, _, _, a := c.RGBA()
	if a == 0 {
		t.Errorf("expected non-transparent at (35,35) after negative translation, got %v", c)
	}
}

func TestAlignFramesNoDrift(t *testing.T) {
	img1 := newTestImage(512, 512, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 200, 100, 300, 400, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(512, 512, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 200, 100, 300, 400, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 200, MinY: 100, MaxX: 299, MaxY: 399, Width: 100, Height: 300, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 200, MinY: 100, MaxX: 299, MaxY: 399, Width: 100, Height: 300, Empty: false}

	d := NewDriftDetector()
	results, err := d.AlignFrames([]image.Image{img1, img2}, []backgroundremoval.SubjectBox{box1, box2}, DefaultFeetCenterAnchor)
	if err != nil {
		t.Fatalf("AlignFrames failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].DriftDetected {
		t.Errorf("first frame should not drift, flags %v", results[0].QualityFlags)
	}
	if results[1].DriftDetected {
		t.Errorf("expected no drift, got flags %v type %s", results[1].QualityFlags, results[1].DriftType)
	}
	if results[1].AlignedImage == nil {
		t.Errorf("expected aligned image")
	}
}

func TestAlignFramesHorizontalDrift(t *testing.T) {
	img1 := newTestImage(512, 512, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 200, 100, 300, 400, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(512, 512, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 230, 100, 330, 400, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 200, MinY: 100, MaxX: 299, MaxY: 399, Width: 100, Height: 300, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 230, MinY: 100, MaxX: 329, MaxY: 399, Width: 100, Height: 300, Empty: false}

	d := NewDriftDetector()
	results, err := d.AlignFrames([]image.Image{img1, img2}, []backgroundremoval.SubjectBox{box1, box2}, DefaultFeetCenterAnchor)
	if err != nil {
		t.Fatalf("AlignFrames failed: %v", err)
	}
	if !results[1].DriftDetected {
		t.Errorf("expected drift detection")
	}
	found := false
	for _, f := range results[1].QualityFlags {
		if f == FlagAnchorDrift {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ANCHOR_DRIFT flag, got %v", results[1].QualityFlags)
	}
	excessive := false
	for _, f := range results[1].QualityFlags {
		if f == FlagExcessiveFrameDrift {
			excessive = true
		}
	}
	if excessive {
		t.Errorf("should not be excessive for small drift")
	}
}

func TestAlignFramesExcessiveDrift(t *testing.T) {
	img1 := newTestImage(512, 512, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 200, 100, 300, 400, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(512, 512, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 400, 100, 500, 400, color.NRGBA{0, 255, 0, 255})

	box1 := backgroundremoval.SubjectBox{MinX: 200, MinY: 100, MaxX: 299, MaxY: 399, Width: 100, Height: 300, Empty: false}
	box2 := backgroundremoval.SubjectBox{MinX: 400, MinY: 100, MaxX: 499, MaxY: 399, Width: 100, Height: 300, Empty: false}

	d := NewDriftDetector()
	results, err := d.AlignFrames([]image.Image{img1, img2}, []backgroundremoval.SubjectBox{box1, box2}, DefaultFeetCenterAnchor)
	if err != nil {
		t.Fatalf("AlignFrames failed: %v", err)
	}
	found := false
	for _, f := range results[1].QualityFlags {
		if f == FlagExcessiveFrameDrift {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EXCESSIVE_FRAME_DRIFT flag, got %v", results[1].QualityFlags)
	}
	if results[1].AlignedImage != img2 {
		t.Errorf("excessive drift should not modify image (no repair)")
	}
}

func TestAlignDetectDriftScale(t *testing.T) {
	d := NewDriftDetector()
	cfg := DefaultCanvasConfig()
	refBox := backgroundremoval.SubjectBox{MinX: 0, MinY: 0, MaxX: 99, MaxY: 199, Width: 100, Height: 200, Empty: false}
	curBox := backgroundremoval.SubjectBox{MinX: 0, MinY: 0, MaxX: 99, MaxY: 259, Width: 100, Height: 260, Empty: false}
	refAnchor := Anchor{Type: AnchorFeetCenter, X: 50, Y: 200}
	curAnchor := Anchor{Type: AnchorFeetCenter, X: 50, Y: 260}

	driftType, flags := d.detectDrift(curAnchor, refAnchor, curBox, refBox, cfg)
	scaleFlag := false
	for _, f := range flags {
		if f == FlagScaleDrift {
			scaleFlag = true
		}
	}
	if !scaleFlag {
		t.Errorf("expected SCALE_DRIFT flag, got %v type %s", flags, driftType)
	}
}

func TestAlignDetectDriftFeet(t *testing.T) {
	d := NewDriftDetector()
	cfg := DefaultCanvasConfig()
	refBox := backgroundremoval.SubjectBox{MinX: 100, MinY: 100, MaxX: 199, MaxY: 299, Width: 100, Height: 200, Empty: false}
	curBox := backgroundremoval.SubjectBox{MinX: 150, MinY: 150, MaxX: 249, MaxY: 349, Width: 100, Height: 200, Empty: false}
	refAnchor := Anchor{Type: AnchorFeetCenter, X: 150, Y: 299}
	curAnchor := Anchor{Type: AnchorFeetCenter, X: 200, Y: 349}

	driftType, flags := d.detectDrift(curAnchor, refAnchor, curBox, refBox, cfg)
	if driftType != "feet" {
		t.Errorf("expected feet drift, got %s", driftType)
	}
	found := false
	for _, f := range flags {
		if f == FlagAnchorDrift {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ANCHOR_DRIFT flag, got %v", flags)
	}
}

func TestAlignDetectDriftNone(t *testing.T) {
	d := NewDriftDetector()
	cfg := DefaultCanvasConfig()
	box := backgroundremoval.SubjectBox{MinX: 100, MinY: 100, MaxX: 199, MaxY: 299, Width: 100, Height: 200, Empty: false}
	anchor := Anchor{Type: AnchorFeetCenter, X: 150, Y: 299}

	driftType, flags := d.detectDrift(anchor, anchor, box, box, cfg)
	if driftType != "none" {
		t.Errorf("expected no drift, got %s", driftType)
	}
	if len(flags) != 0 {
		t.Errorf("expected no flags, got %v", flags)
	}
}

func TestAlignFramesMismatch(t *testing.T) {
	d := NewDriftDetector()
	_, err := d.AlignFrames(
		[]image.Image{newTestImage(10, 10, color.NRGBA{0, 0, 0, 0})},
		[]backgroundremoval.SubjectBox{{Empty: true}, {Empty: true}},
		DefaultFeetCenterAnchor,
	)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestAlignFramesEmpty(t *testing.T) {
	d := NewDriftDetector()
	_, err := d.AlignFrames(nil, nil, DefaultFeetCenterAnchor)
	if err == nil {
		t.Fatal("expected empty error")
	}
}
