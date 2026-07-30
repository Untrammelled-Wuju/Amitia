// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"
	"math"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type MotionDetector struct{}

func NewMotionDetector() *MotionDetector {
	return &MotionDetector{}
}

func (d *MotionDetector) Key() string {
	return "motion"
}

func (d *MotionDetector) Version() string {
	return "1.0"
}

func (d *MotionDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
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
	maxJump := input.Profile.MotionPolicy.MaxMotionJump
	if maxJump <= 0 {
		maxJump = 0.1
	}
	var observations []quality.Observation
	xVelocities := make([]float64, 0, len(frames)-1)
	yVelocities := make([]float64, 0, len(frames)-1)
	for i := 0; i < len(frames)-1; i++ {
		cur := frames[i]
		next := frames[i+1]
		dx := next.CentroidX - cur.CentroidX
		dy := next.CentroidY - cur.CentroidY
		motionJump := math.Sqrt(dx*dx+dy*dy) / canvasDiag
		boxWChange := 0.0
		boxHChange := 0.0
		if cur.SubjectBoxWidth > 0 {
			boxWChange = math.Abs(next.SubjectBoxWidth-cur.SubjectBoxWidth) / cur.SubjectBoxWidth
		}
		if cur.SubjectBoxHeight > 0 {
			boxHChange = math.Abs(next.SubjectBoxHeight-cur.SubjectBoxHeight) / cur.SubjectBoxHeight
		}
		fgChange := math.Abs(next.ForegroundCoverage - cur.ForegroundCoverage)
		xVelocities = append(xVelocities, dx)
		yVelocities = append(yVelocities, dy)
		if motionJump > maxJump {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      cur.FrameIndex,
				FramePairFrom:   cur.FrameIndex,
				FramePairTo:     next.FrameIndex,
				MetricName:      "motion_jump",
				Value:           motionJump,
				Confidence:      0.85,
				Details: map[string]float64{
					"dx":                dx,
					"dy":                dy,
					"motion_jump":       motionJump,
					"box_width_change":  boxWChange,
					"box_height_change": boxHChange,
					"foreground_change": fgChange,
				},
			})
		}
	}
	policy := input.Profile.MotionPolicy
	if policy.AllowHorizontalMotion {
		reversals, lastFrame := countSignReversals(xVelocities, frames)
		if reversals > 2 {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      lastFrame,
				MetricName:      "motion_direction_reversal",
				Value:           float64(reversals),
				Confidence:      0.7,
				Details: map[string]float64{
					"axis":           1,
					"reversal_count": float64(reversals),
				},
			})
		}
	}
	if policy.AllowVerticalMotion {
		reversals, lastFrame := countSignReversals(yVelocities, frames)
		if reversals > 2 {
			observations = append(observations, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      lastFrame,
				MetricName:      "motion_direction_reversal",
				Value:           float64(reversals),
				Confidence:      0.7,
				Details: map[string]float64{
					"axis":           2,
					"reversal_count": float64(reversals),
				},
			})
		}
	}
	return observations, nil
}

func countSignReversals(velocities []float64, frames []quality.FrameMeasurement) (int, int) {
	count := 0
	lastSign := 0
	lastFrame := 0
	for i, v := range velocities {
		sign := 0
		if v > 0 {
			sign = 1
		} else if v < 0 {
			sign = -1
		}
		if sign != 0 {
			if lastSign != 0 && sign != lastSign {
				count++
				if i+1 < len(frames) {
					lastFrame = frames[i+1].FrameIndex
				}
			}
			lastSign = sign
		}
	}
	return count, lastFrame
}
