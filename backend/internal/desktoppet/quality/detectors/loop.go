// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"
	"math"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type LoopDetector struct{}

func NewLoopDetector() *LoopDetector {
	return &LoopDetector{}
}

func (d *LoopDetector) Key() string {
	return "loop"
}

func (d *LoopDetector) Version() string {
	return "1.0"
}

func (d *LoopDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.Measurements == nil {
		return nil, nil
	}
	ms := input.Measurements
	frames := ms.FrameMeasurements
	if len(frames) < 3 {
		return nil, nil
	}
	loopType := ms.LoopType
	if loopType != "loop" && loopType != "ping_pong" {
		return nil, nil
	}
	canvasW := float64(ms.CanvasWidth)
	canvasH := float64(ms.CanvasHeight)
	if canvasW <= 0 {
		canvasW = 1
	}
	if canvasH <= 0 {
		canvasH = 1
	}
	canvasDiag := math.Sqrt(canvasW*canvasW + canvasH*canvasH)
	if canvasDiag <= 0 {
		canvasDiag = 1
	}
	first := frames[0]
	last := frames[len(frames)-1]
	prev := frames[len(frames)-2]
	dx := first.CentroidX - last.CentroidX
	dy := first.CentroidY - last.CentroidY
	coverageDiff := math.Abs(last.ForegroundCoverage - first.ForegroundCoverage)
	posDiscontinuity := math.Sqrt(dx*dx+dy*dy) / canvasDiag
	boxWChange := math.Abs(last.SubjectBoxWidth - first.SubjectBoxWidth)
	boxHChange := math.Abs(last.SubjectBoxHeight - first.SubjectBoxHeight)
	loopDiscontinuity := posDiscontinuity + coverageDiff
	var observations []quality.Observation
	observations = append(observations, quality.Observation{
		DetectorKey:     d.Key(),
		DetectorVersion: d.Version(),
		FrameIndex:      last.FrameIndex,
		FramePairFrom:   last.FrameIndex,
		FramePairTo:     first.FrameIndex,
		MetricName:      "loop_discontinuity",
		Value:           loopDiscontinuity,
		Confidence:      0.8,
		Details: map[string]float64{
			"dx":                dx,
			"dy":                dy,
			"pos_discontinuity": posDiscontinuity,
			"coverage_diff":     coverageDiff,
			"box_width_change":  boxWChange,
			"box_height_change": boxHChange,
		},
	})
	tailVx := last.CentroidX - prev.CentroidX
	tailVy := last.CentroidY - prev.CentroidY
	wrapMag := math.Sqrt(dx*dx + dy*dy)
	tailMag := math.Sqrt(tailVx*tailVx + tailVy*tailVy)
	velocityDiscontinuity := 0.0
	cosSim := 1.0
	if wrapMag > 1e-9 && tailMag > 1e-9 {
		cosSim = (dx*tailVx + dy*tailVy) / (wrapMag * tailMag)
		if cosSim > 1 {
			cosSim = 1
		} else if cosSim < -1 {
			cosSim = -1
		}
		velocityDiscontinuity = (1 - cosSim) / 2
	}
	observations = append(observations, quality.Observation{
		DetectorKey:     d.Key(),
		DetectorVersion: d.Version(),
		FrameIndex:      last.FrameIndex,
		FramePairFrom:   last.FrameIndex,
		FramePairTo:     first.FrameIndex,
		MetricName:      "loop_velocity_discontinuity",
		Value:           velocityDiscontinuity,
		Confidence:      0.8,
		Details: map[string]float64{
			"wrap_vx":        dx,
			"wrap_vy":        dy,
			"tail_vx":        tailVx,
			"tail_vy":        tailVy,
			"cos_similarity": cosSim,
		},
	})
	return observations, nil
}
