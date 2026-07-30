// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"
	"math"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type ColorDetector struct {
	flickerThreshold float64
}

func NewColorDetector() *ColorDetector {
	return &ColorDetector{flickerThreshold: 0.15}
}

func (d *ColorDetector) Key() string {
	return "color"
}

func (d *ColorDetector) Version() string {
	return "1.0"
}

func (d *ColorDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
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
	threshold := d.flickerThreshold
	if threshold <= 0 {
		threshold = 0.15
	}
	var observations []quality.Observation
	for i := 0; i < len(frames)-1; i++ {
		cur := frames[i]
		next := frames[i+1]
		fgChange := math.Abs(next.ForegroundCoverage - cur.ForegroundCoverage)
		opaqueChange := math.Abs(next.OpaqueCoverage - cur.OpaqueCoverage)
		if fgChange > threshold || opaqueChange > threshold {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      next.FrameIndex,
				FramePairFrom:   cur.FrameIndex,
				FramePairTo:     next.FrameIndex,
				MetricName:      "color_flicker",
				Value:           math.Max(fgChange, opaqueChange),
				Confidence:      0.6,
				Details: map[string]float64{
					"foreground_coverage_change": fgChange,
					"opaque_coverage_change":     opaqueChange,
				},
			})
		}
	}
	return observations, nil
}
