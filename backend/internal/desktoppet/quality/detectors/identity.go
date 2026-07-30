// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"
	"math"
	"sort"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type IdentityDetector struct {
	driftThreshold float64
}

func NewIdentityDetector() *IdentityDetector {
	return &IdentityDetector{driftThreshold: 0.15}
}

func (d *IdentityDetector) Key() string {
	return "identity"
}

func (d *IdentityDetector) Version() string {
	return "1.0"
}

func (d *IdentityDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.Measurements == nil {
		return nil, nil
	}
	ms := input.Measurements
	frames := ms.FrameMeasurements
	if len(frames) < 2 {
		return nil, nil
	}
	threshold := d.driftThreshold
	if threshold <= 0 {
		threshold = 0.15
	}
	canvasW := float64(ms.CanvasWidth)
	canvasH := float64(ms.CanvasHeight)
	if canvasW <= 0 {
		canvasW = 1
	}
	if canvasH <= 0 {
		canvasH = 1
	}
	n := len(frames)
	aspectRatios := make([]float64, n)
	fgCoverages := make([]float64, n)
	cxNorms := make([]float64, n)
	cyNorms := make([]float64, n)
	for i, f := range frames {
		h := f.SubjectBoxHeight
		if h <= 0 {
			h = 1
		}
		aspectRatios[i] = f.SubjectBoxWidth / h
		fgCoverages[i] = f.ForegroundCoverage
		cxNorms[i] = f.CentroidX / canvasW
		cyNorms[i] = f.CentroidY / canvasH
	}
	medianAspect := medianFloat64(aspectRatios)
	medianFg := medianFloat64(fgCoverages)
	medianCx := medianFloat64(cxNorms)
	medianCy := medianFloat64(cyNorms)
	aspectScale := medianAspect
	if aspectScale < 1 {
		aspectScale = 1
	}
	var observations []quality.Observation
	for _, f := range frames {
		h := f.SubjectBoxHeight
		if h <= 0 {
			h = 1
		}
		aspect := f.SubjectBoxWidth / h
		aspectDiff := math.Abs(aspect-medianAspect) / aspectScale
		fgDiff := math.Abs(f.ForegroundCoverage - medianFg)
		cxDiff := math.Abs(f.CentroidX/canvasW - medianCx)
		cyDiff := math.Abs(f.CentroidY/canvasH - medianCy)
		distance := math.Sqrt(aspectDiff*aspectDiff + fgDiff*fgDiff + cxDiff*cxDiff + cyDiff*cyDiff)
		if distance > threshold {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      "identity_drift",
				Value:           distance,
				Confidence:      0.7,
				Details: map[string]float64{
					"aspect_ratio":        aspect,
					"foreground_coverage": f.ForegroundCoverage,
					"centroid_x_norm":     f.CentroidX / canvasW,
					"centroid_y_norm":     f.CentroidY / canvasH,
					"distance":            distance,
				},
			})
		}
	}
	return observations, nil
}

func medianFloat64(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
