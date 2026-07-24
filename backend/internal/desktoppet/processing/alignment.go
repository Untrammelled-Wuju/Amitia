// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"image"
	"math"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

const (
	FlagAnchorDrift         = "ANCHOR_DRIFT"
	FlagScaleDrift          = "SCALE_DRIFT"
	FlagExcessiveFrameDrift = "EXCESSIVE_FRAME_DRIFT"
)

type AlignmentResult struct {
	AlignedImage  image.Image
	Anchor        Anchor
	DriftDetected bool
	DriftType     string
	QualityFlags  []string
}

type DriftDetector struct {
	maxHorizontalDrift float64
	maxVerticalDrift   float64
	maxScaleChange     float64
}

func NewDriftDetector() *DriftDetector {
	return &DriftDetector{
		maxHorizontalDrift: 0.05,
		maxVerticalDrift:   0.05,
		maxScaleChange:     0.1,
	}
}

func (d *DriftDetector) AlignFrames(imgs []image.Image, boxes []backgroundremoval.SubjectBox, anchor Anchor) ([]AlignmentResult, error) {
	if len(imgs) == 0 {
		return nil, NewProcessingError(ErrCodeSubjectAlignmentFailed, "no images to align")
	}
	if len(imgs) != len(boxes) {
		return nil, NewProcessingError(ErrCodeSubjectAlignmentFailed, "images and boxes length mismatch")
	}

	cfg := DefaultCanvasConfig()

	anchors := make([]Anchor, len(imgs))
	for i, box := range boxes {
		anchors[i] = ComputeAnchor(box, anchor.Type)
	}

	refAnchor := anchors[0]
	refBox := boxes[0]

	results := make([]AlignmentResult, len(imgs))
	for i, img := range imgs {
		curAnchor := anchors[i]
		curBox := boxes[i]

		dx := int(math.Round(refAnchor.X - curAnchor.X))
		dy := int(math.Round(refAnchor.Y - curAnchor.Y))

		driftType, flags := d.detectDrift(curAnchor, refAnchor, curBox, refBox, cfg)

		excessive := false
		for _, f := range flags {
			if f == FlagExcessiveFrameDrift {
				excessive = true
				break
			}
		}

		var aligned image.Image = img
		if !excessive {
			aligned = applyTranslation(img, dx, dy, cfg)
		}

		results[i] = AlignmentResult{
			AlignedImage:  aligned,
			Anchor:        curAnchor,
			DriftDetected: len(flags) > 0,
			DriftType:     driftType,
			QualityFlags:  flags,
		}
	}
	return results, nil
}

func applyTranslation(img image.Image, dx, dy int, cfg CanvasConfig) image.Image {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()

	w := cfg.OutputWidth
	h := cfg.OutputHeight
	if w <= 0 || h <= 0 {
		w = bounds.Dx()
		h = bounds.Dy()
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		dstY := y - bounds.Min.Y + dy
		if dstY < 0 || dstY >= h {
			continue
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dstX := x - bounds.Min.X + dx
			if dstX < 0 || dstX >= w {
				continue
			}
			canvas.Set(dstX, dstY, img.At(x, y))
		}
	}
	return canvas
}

func (d *DriftDetector) detectDrift(currentAnchor, refAnchor Anchor, currentBox, refBox backgroundremoval.SubjectBox, cfg CanvasConfig) (string, []string) {
	var flags []string
	driftType := "none"

	canvasW := float64(cfg.OutputWidth)
	canvasH := float64(cfg.OutputHeight)
	if canvasW <= 0 {
		canvasW = 1
	}
	if canvasH <= 0 {
		canvasH = 1
	}

	hDrift := absFloat(currentAnchor.X-refAnchor.X) / canvasW
	vDrift := absFloat(currentAnchor.Y-refAnchor.Y) / canvasH

	var scaleChange float64
	if refBox.Height > 0 {
		scaleChange = absFloat(float64(currentBox.Height)-float64(refBox.Height)) / float64(refBox.Height)
	}

	hExceeded := hDrift > d.maxHorizontalDrift
	vExceeded := vDrift > d.maxVerticalDrift
	scaleExceeded := scaleChange > d.maxScaleChange

	if hExceeded && vExceeded {
		driftType = "feet"
		flags = append(flags, FlagAnchorDrift)
	} else if hExceeded {
		driftType = "horizontal"
		flags = append(flags, FlagAnchorDrift)
	} else if vExceeded {
		driftType = "vertical"
		flags = append(flags, FlagAnchorDrift)
	}

	if scaleExceeded {
		flags = append(flags, FlagScaleDrift)
		if driftType == "none" {
			driftType = "scale"
		}
	}

	excessiveH := d.maxHorizontalDrift * 2
	excessiveV := d.maxVerticalDrift * 2
	excessiveScale := d.maxScaleChange * 2
	if hDrift > excessiveH || vDrift > excessiveV || scaleChange > excessiveScale {
		flags = append(flags, FlagExcessiveFrameDrift)
		if driftType == "none" {
			driftType = "center"
		}
	}

	return driftType, flags
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
