// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"errors"
	"math"
	"sort"
)

type ScaleResult struct {
	Scale           float64
	BaseScale       float64
	ClampedScale    float64
	ClampReason     string
	ReferenceHeight float64
	ReferenceWidth  float64
}

func ComputeCharacterScaleBaseline(subjectHeights []float64, subjectWidths []float64, canvasW, canvasH int, targetHeightRatio, maxWidthRatio float64, safeMarginT, safeMarginR, safeMarginB, safeMarginL int) (ScaleResult, error) {
	if len(subjectHeights) == 0 {
		return ScaleResult{}, errors.New("no subject heights provided")
	}
	if canvasW <= 0 || canvasH <= 0 {
		return ScaleResult{}, errors.New("invalid canvas dimensions")
	}

	refHeight := Median(subjectHeights)
	var refWidth float64
	if len(subjectWidths) > 0 {
		refWidth = Median(subjectWidths)
	}

	if refHeight <= 0 {
		return ScaleResult{}, errors.New("reference height is zero or negative")
	}

	availableHeight := float64(canvasH) - float64(safeMarginT+safeMarginB)
	targetHeight := float64(canvasH) * targetHeightRatio
	if targetHeight > availableHeight && availableHeight > 0 {
		targetHeight = availableHeight
	}

	scale := targetHeight / refHeight

	clampedScale := scale
	clampReason := ""

	availableWidth := float64(canvasW) - float64(safeMarginL+safeMarginR)
	maxWidth := float64(canvasW) * maxWidthRatio
	if maxWidth > availableWidth && availableWidth > 0 {
		maxWidth = availableWidth
	}

	if refWidth > 0 && maxWidth > 0 {
		scaledWidth := refWidth * scale
		if scaledWidth > maxWidth {
			widthScale := maxWidth / refWidth
			clampedScale = widthScale
			clampReason = "width_clamp"
		}
	}

	return ScaleResult{
		Scale:           scale,
		BaseScale:       scale,
		ClampedScale:    clampedScale,
		ClampReason:     clampReason,
		ReferenceHeight: refHeight,
		ReferenceWidth:  refWidth,
	}, nil
}

func ComputeActionScale(baselineScale float64, actionSubjectHeight, actionSubjectWidth float64, canvasW, canvasH int, targetHeightRatio, maxWidthRatio float64, safeMarginT, safeMarginR, safeMarginB, safeMarginL int, maxDelta float64) (ScaleResult, error) {
	if canvasW <= 0 || canvasH <= 0 {
		return ScaleResult{}, errors.New("invalid canvas dimensions")
	}
	if actionSubjectHeight <= 0 {
		return ScaleResult{}, errors.New("action subject height is zero or negative")
	}

	availableHeight := float64(canvasH) - float64(safeMarginT+safeMarginB)
	targetHeight := float64(canvasH) * targetHeightRatio
	if targetHeight > availableHeight && availableHeight > 0 {
		targetHeight = availableHeight
	}

	scale := targetHeight / actionSubjectHeight

	availableWidth := float64(canvasW) - float64(safeMarginL+safeMarginR)
	maxWidth := float64(canvasW) * maxWidthRatio
	if maxWidth > availableWidth && availableWidth > 0 {
		maxWidth = availableWidth
	}

	clampedScale := scale
	clampReason := ""

	if actionSubjectWidth > 0 && maxWidth > 0 {
		scaledWidth := actionSubjectWidth * scale
		if scaledWidth > maxWidth {
			widthScale := maxWidth / actionSubjectWidth
			clampedScale = widthScale
			clampReason = "width_clamp"
		}
	}

	if maxDelta > 0 && baselineScale > 0 {
		minScale := baselineScale * (1.0 - maxDelta)
		maxScale := baselineScale * (1.0 + maxDelta)
		if clampedScale < minScale {
			clampedScale = minScale
			if clampReason != "" {
				clampReason += ";"
			}
			clampReason += "delta_clamp_low"
		} else if clampedScale > maxScale {
			clampedScale = maxScale
			if clampReason != "" {
				clampReason += ";"
			}
			clampReason += "delta_clamp_high"
		}
	}

	return ScaleResult{
		Scale:           scale,
		BaseScale:       baselineScale,
		ClampedScale:    clampedScale,
		ClampReason:     clampReason,
		ReferenceHeight: actionSubjectHeight,
		ReferenceWidth:  actionSubjectWidth,
	}, nil
}

func Median(values []float64) float64 {
	return Percentile(values, 0.5)
}

func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	if len(sorted) == 1 {
		return sorted[0]
	}

	rank := p * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return sorted[lower]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
