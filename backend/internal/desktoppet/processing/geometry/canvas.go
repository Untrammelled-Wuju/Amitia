// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"errors"
	"strings"
)

type CanvasMappingResult struct {
	Transform     AffineTransform
	ClippedPixels int
	ClippedSides  string
	VisibleBox    PixelRect
}

func MapToCanvas(sourceAnchor PixelPoint, scale float64, targetAnchor NormalizedPoint, canvasW, canvasH int, subjectBox PixelRect) (*CanvasMappingResult, error) {
	if canvasW <= 0 || canvasH <= 0 {
		return nil, errors.New("invalid canvas dimensions")
	}
	if scale <= 0 {
		return nil, errors.New("scale must be positive")
	}

	sourceAnchorScaledX := sourceAnchor.X * scale
	sourceAnchorScaledY := sourceAnchor.Y * scale

	targetPixelX := targetAnchor.X * float64(canvasW)
	targetPixelY := targetAnchor.Y * float64(canvasH)

	tx := targetPixelX - sourceAnchorScaledX
	ty := targetPixelY - sourceAnchorScaledY

	scaleT := NewScaleTransform(sourceAnchor.Space, SpaceScaled, scale, scale)
	translateT := NewTranslationTransform(SpaceScaled, SpaceCanvas, tx, ty)
	transform := translateT.Compose(scaleT)

	scaledBox := scaleT.ApplyRect(subjectBox)
	unclippedBox := translateT.ApplyRect(scaledBox)

	var clippedSides []string
	if unclippedBox.MinX < 0 {
		clippedSides = append(clippedSides, "left")
	}
	if unclippedBox.MinY < 0 {
		clippedSides = append(clippedSides, "top")
	}
	if unclippedBox.MaxX > canvasW {
		clippedSides = append(clippedSides, "right")
	}
	if unclippedBox.MaxY > canvasH {
		clippedSides = append(clippedSides, "bottom")
	}

	visibleBox := unclippedBox
	if visibleBox.MinX < 0 {
		visibleBox.MinX = 0
	}
	if visibleBox.MinY < 0 {
		visibleBox.MinY = 0
	}
	if visibleBox.MaxX > canvasW {
		visibleBox.MaxX = canvasW
	}
	if visibleBox.MaxY > canvasH {
		visibleBox.MaxY = canvasH
	}
	if visibleBox.MinX > visibleBox.MaxX {
		visibleBox.MaxX = visibleBox.MinX
	}
	if visibleBox.MinY > visibleBox.MaxY {
		visibleBox.MaxY = visibleBox.MinY
	}
	visibleBox.Space = SpaceCanvas

	unclippedArea := unclippedBox.Area()
	visibleArea := visibleBox.Area()
	clippedPixels := unclippedArea - visibleArea
	if clippedPixels < 0 {
		clippedPixels = 0
	}

	clippedSidesStr := strings.Join(clippedSides, ",")

	return &CanvasMappingResult{
		Transform:     transform,
		ClippedPixels: clippedPixels,
		ClippedSides:  clippedSidesStr,
		VisibleBox:    visibleBox,
	}, nil
}
