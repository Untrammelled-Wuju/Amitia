// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"errors"
	"image"
)

type AnchorMode string

const (
	AnchorFeetCenter        AnchorMode = "feet_center"
	AnchorBodyCenter        AnchorMode = "body_center"
	AnchorHeadCenter        AnchorMode = "head_center"
	AnchorHandContact       AnchorMode = "hand_contact"
	AnchorWindowEdgeContact AnchorMode = "window_edge_contact"
)

type SourceAnchorResult struct {
	Point      PixelPoint
	Mode       AnchorMode
	Confidence float64
	Estimated  bool
	Method     string
}

func EstimateSourceAnchor(analysis *SubjectAnalysis, mask *image.Gray, mode AnchorMode, space CoordinateSpaceID) (*SourceAnchorResult, error) {
	if analysis == nil {
		return nil, errors.New("analysis is nil")
	}
	if analysis.MainComponent == nil {
		return nil, errors.New("no main component in analysis")
	}

	main := analysis.MainComponent
	box := main.Box

	switch mode {
	case AnchorFeetCenter:
		return estimateFeetCenter(mask, box, space), nil
	case AnchorBodyCenter:
		return estimateBodyCenter(main, space), nil
	case AnchorHeadCenter:
		return estimateHeadCenter(box, space), nil
	case AnchorHandContact:
		return estimateHandContact(mask, box, space), nil
	case AnchorWindowEdgeContact:
		return estimateWindowEdge(mask, box, space), nil
	default:
		return nil, errors.New("unknown anchor mode: " + string(mode))
	}
}

func estimateFeetCenter(mask *image.Gray, box PixelRect, space CoordinateSpaceID) *SourceAnchorResult {
	bandHeight := int(float64(box.Height()) * 0.1)
	if bandHeight < 1 {
		bandHeight = 1
	}
	bandMinY := box.MaxY - bandHeight
	if bandMinY < box.MinY {
		bandMinY = box.MinY
	}

	bounds := mask.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	stride := mask.Stride
	pix := mask.Pix

	var xs []float64
	lowestY := float64(bandMinY)

	for y := bandMinY; y < box.MaxY && y < h; y++ {
		if y < 0 {
			continue
		}
		rowBase := y * stride
		for x := box.MinX; x < box.MaxX && x < w; x++ {
			if x < 0 {
				continue
			}
			if pix[rowBase+x] > 0 {
				xs = append(xs, float64(x))
				if float64(y) > lowestY {
					lowestY = float64(y)
				}
			}
		}
	}

	if len(xs) == 0 {
		cx, _ := box.Center()
		return &SourceAnchorResult{
			Point:      PixelPoint{X: cx, Y: float64(box.MaxY), Space: space},
			Mode:       AnchorFeetCenter,
			Confidence: 0.3,
			Estimated:  true,
			Method:     "feet_center_fallback_box",
		}
	}

	medianX := Median(xs)
	return &SourceAnchorResult{
		Point:      PixelPoint{X: medianX, Y: lowestY, Space: space},
		Mode:       AnchorFeetCenter,
		Confidence: 0.9,
		Estimated:  false,
		Method:     "feet_center_bottom_band",
	}
}

func estimateBodyCenter(main *ComponentInfo, space CoordinateSpaceID) *SourceAnchorResult {
	return &SourceAnchorResult{
		Point:      PixelPoint{X: main.CentroidX, Y: main.CentroidY, Space: space},
		Mode:       AnchorBodyCenter,
		Confidence: 0.85,
		Estimated:  false,
		Method:     "body_center_centroid",
	}
}

func estimateHeadCenter(box PixelRect, space CoordinateSpaceID) *SourceAnchorResult {
	cx, _ := box.Center()
	upperY := float64(box.MinY) + float64(box.Height())*0.1
	return &SourceAnchorResult{
		Point:      PixelPoint{X: cx, Y: upperY, Space: space},
		Mode:       AnchorHeadCenter,
		Confidence: 0.5,
		Estimated:  true,
		Method:     "head_center_upper_estimate",
	}
}

func estimateHandContact(mask *image.Gray, box PixelRect, space CoordinateSpaceID) *SourceAnchorResult {
	bounds := mask.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	stride := mask.Stride
	pix := mask.Pix

	minY := h
	var topXs []float64

	for y := box.MinY; y < box.MaxY && y < h; y++ {
		if y < 0 {
			continue
		}
		rowBase := y * stride
		for x := box.MinX; x < box.MaxX && x < w; x++ {
			if x < 0 {
				continue
			}
			if pix[rowBase+x] > 0 {
				if y < minY {
					minY = y
					topXs = nil
					topXs = append(topXs, float64(x))
				} else if y == minY {
					topXs = append(topXs, float64(x))
				}
			}
		}
	}

	if len(topXs) == 0 {
		cx, _ := box.Center()
		return &SourceAnchorResult{
			Point:      PixelPoint{X: cx, Y: float64(box.MinY), Space: space},
			Mode:       AnchorHandContact,
			Confidence: 0.3,
			Estimated:  true,
			Method:     "hand_contact_fallback_box",
		}
	}

	medianX := Median(topXs)
	return &SourceAnchorResult{
		Point:      PixelPoint{X: medianX, Y: float64(minY), Space: space},
		Mode:       AnchorHandContact,
		Confidence: 0.5,
		Estimated:  true,
		Method:     "hand_contact_top_extreme",
	}
}

func estimateWindowEdge(mask *image.Gray, box PixelRect, space CoordinateSpaceID) *SourceAnchorResult {
	bounds := mask.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	stride := mask.Stride
	pix := mask.Pix

	maxY := 0
	var bottomXs []float64

	for y := box.MinY; y < box.MaxY && y < h; y++ {
		if y < 0 {
			continue
		}
		rowBase := y * stride
		for x := box.MinX; x < box.MaxX && x < w; x++ {
			if x < 0 {
				continue
			}
			if pix[rowBase+x] > 0 {
				if y > maxY {
					maxY = y
					bottomXs = nil
					bottomXs = append(bottomXs, float64(x))
				} else if y == maxY {
					bottomXs = append(bottomXs, float64(x))
				}
			}
		}
	}

	if len(bottomXs) == 0 {
		cx, _ := box.Center()
		return &SourceAnchorResult{
			Point:      PixelPoint{X: cx, Y: float64(box.MaxY), Space: space},
			Mode:       AnchorWindowEdgeContact,
			Confidence: 0.3,
			Estimated:  true,
			Method:     "window_edge_fallback_box",
		}
	}

	medianX := Median(bottomXs)
	return &SourceAnchorResult{
		Point:      PixelPoint{X: medianX, Y: float64(maxY), Space: space},
		Mode:       AnchorWindowEdgeContact,
		Confidence: 0.5,
		Estimated:  true,
		Method:     "window_edge_bottom_extreme",
	}
}
