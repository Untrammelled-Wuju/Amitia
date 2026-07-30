// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"
	"math"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type DuplicateDetector struct {
	perceptualThreshold float64
	frozenMinPairs      int
}

func NewDuplicateDetector() *DuplicateDetector {
	return &DuplicateDetector{
		perceptualThreshold: 0.98,
		frozenMinPairs:      3,
	}
}

func (d *DuplicateDetector) Key() string {
	return "duplicate"
}

func (d *DuplicateDetector) Version() string {
	return "1.0"
}

func (d *DuplicateDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
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
	perceptualThreshold := d.perceptualThreshold
	if perceptualThreshold <= 0 {
		perceptualThreshold = 0.98
	}
	frozenMinPairs := d.frozenMinPairs
	if frozenMinPairs <= 0 {
		frozenMinPairs = 3
	}
	var observations []quality.Observation
	frozenRunStart := -1
	frozenRunLen := 0
	for i := 0; i < len(frames)-1; i++ {
		cur := frames[i]
		next := frames[i+1]
		isExact := cur.PixelHash != "" && cur.PixelHash == next.PixelHash
		similarity := perceptualSimilarity(cur, next)
		isPerceptual := !isExact && similarity >= perceptualThreshold
		isSimilar := isExact || isPerceptual
		if isExact {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      next.FrameIndex,
				FramePairFrom:   cur.FrameIndex,
				FramePairTo:     next.FrameIndex,
				MetricName:      "exact_duplicate",
				Value:           1.0,
				Confidence:      0.9,
				Details: map[string]float64{
					"similarity": similarity,
				},
			})
		} else if isPerceptual {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      next.FrameIndex,
				FramePairFrom:   cur.FrameIndex,
				FramePairTo:     next.FrameIndex,
				MetricName:      "perceptual_duplicate",
				Value:           similarity,
				Confidence:      0.6,
				Details: map[string]float64{
					"similarity": similarity,
				},
			})
		}
		if isSimilar {
			if frozenRunStart < 0 {
				frozenRunStart = i
				frozenRunLen = 1
			} else {
				frozenRunLen++
			}
		} else {
			if frozenRunLen >= frozenMinPairs {
				observations = append(observations, buildFrozenObservation(d, frames, frozenRunStart, frozenRunLen))
			}
			frozenRunStart = -1
			frozenRunLen = 0
		}
	}
	if frozenRunLen >= frozenMinPairs {
		observations = append(observations, buildFrozenObservation(d, frames, frozenRunStart, frozenRunLen))
	}
	return observations, nil
}

func buildFrozenObservation(d *DuplicateDetector, frames []quality.FrameMeasurement, start, runLen int) quality.Observation {
	return quality.Observation{
		DetectorKey:     d.Key(),
		DetectorVersion: d.Version(),
		FrameIndex:      frames[start].FrameIndex,
		FramePairFrom:   frames[start].FrameIndex,
		FramePairTo:     frames[start+runLen].FrameIndex,
		MetricName:      "frozen_sequence",
		Value:           float64(runLen + 1),
		Confidence:      0.75,
		Details: map[string]float64{
			"frame_count": float64(runLen + 1),
			"pair_count":  float64(runLen),
			"start_frame": float64(frames[start].FrameIndex),
			"end_frame":   float64(frames[start+runLen].FrameIndex),
		},
	}
}

func perceptualSimilarity(a, b quality.FrameMeasurement) float64 {
	coverageDiff := math.Abs(a.ForegroundCoverage - b.ForegroundCoverage)
	maskBase := math.Max(a.MaskArea, b.MaskArea)
	if maskBase < 1e-9 {
		maskBase = 1e-9
	}
	maskDiff := math.Abs(a.MaskArea-b.MaskArea) / maskBase
	boxWBase := math.Max(a.SubjectBoxWidth, b.SubjectBoxWidth)
	boxHBase := math.Max(a.SubjectBoxHeight, b.SubjectBoxHeight)
	if boxWBase < 1e-9 {
		boxWBase = 1e-9
	}
	if boxHBase < 1e-9 {
		boxHBase = 1e-9
	}
	boxWDiff := math.Abs(a.SubjectBoxWidth-b.SubjectBoxWidth) / boxWBase
	boxHDiff := math.Abs(a.SubjectBoxHeight-b.SubjectBoxHeight) / boxHBase
	boxDiff := (boxWDiff + boxHDiff) / 2
	distance := math.Sqrt(coverageDiff*coverageDiff + maskDiff*maskDiff + boxDiff*boxDiff)
	if distance > 1 {
		distance = 1
	}
	return 1 - distance
}
