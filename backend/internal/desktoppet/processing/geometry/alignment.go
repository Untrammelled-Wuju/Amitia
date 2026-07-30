// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"errors"
	"math"
)

type MotionAllowance struct {
	AllowTranslationX  bool
	AllowTranslationY  bool
	AllowScaleChange   bool
	MaxStabilizationX  float64
	MaxStabilizationY  float64
	MaxScaleCorrection float64
	ReferenceStrategy  string
}

type AlignmentMeasurement struct {
	OriginalAnchor  PixelPoint
	CorrectedAnchor PixelPoint
	CorrectionX     float64
	CorrectionY     float64
	Clamped         bool
	ClampReason     string
	ReferenceFrame  int
}

func StabilizeSequence(anchors []PixelPoint, confidences []float64, allowance MotionAllowance, canvasW, canvasH int) ([]AlignmentMeasurement, error) {
	if len(anchors) == 0 {
		return nil, errors.New("no anchors provided")
	}
	if len(anchors) != len(confidences) {
		return nil, errors.New("anchors and confidences length mismatch")
	}

	confThreshold := 0.5
	var refXs, refYs []float64

	for i, a := range anchors {
		if confidences[i] >= confThreshold {
			refXs = append(refXs, a.X)
			refYs = append(refYs, a.Y)
		}
	}

	var refX, refY float64
	if len(refXs) > 0 {
		refX = Median(refXs)
		refY = Median(refYs)
	} else {
		refX = anchors[0].X
		refY = anchors[0].Y
	}

	measurements := make([]AlignmentMeasurement, len(anchors))
	for i, a := range anchors {
		correctedX := a.X
		correctedY := a.Y
		correctionX := 0.0
		correctionY := 0.0
		clamped := false
		clampReason := ""

		if allowance.AllowTranslationX {
			correctionX = refX - a.X
			if allowance.MaxStabilizationX > 0 && math.Abs(correctionX) > allowance.MaxStabilizationX {
				if correctionX > 0 {
					correctionX = allowance.MaxStabilizationX
				} else {
					correctionX = -allowance.MaxStabilizationX
				}
				clamped = true
				clampReason = "x_clamp"
			}
			correctedX = a.X + correctionX
		}

		if allowance.AllowTranslationY {
			correctionY = refY - a.Y
			if allowance.MaxStabilizationY > 0 && math.Abs(correctionY) > allowance.MaxStabilizationY {
				if correctionY > 0 {
					correctionY = allowance.MaxStabilizationY
				} else {
					correctionY = -allowance.MaxStabilizationY
				}
				clamped = true
				if clampReason != "" {
					clampReason += ";"
				}
				clampReason += "y_clamp"
			}
			correctedY = a.Y + correctionY
		}

		measurements[i] = AlignmentMeasurement{
			OriginalAnchor:  a,
			CorrectedAnchor: PixelPoint{X: correctedX, Y: correctedY, Space: a.Space},
			CorrectionX:     correctionX,
			CorrectionY:     correctionY,
			Clamped:         clamped,
			ClampReason:     clampReason,
			ReferenceFrame:  -1,
		}
	}

	return measurements, nil
}
