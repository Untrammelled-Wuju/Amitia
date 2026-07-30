// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"
	"math"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type StabilityDetector struct{}

var _ quality.Detector = (*StabilityDetector)(nil)

func NewStabilityDetector() *StabilityDetector {
	return &StabilityDetector{}
}

func (d *StabilityDetector) Key() string     { return "stability" }
func (d *StabilityDetector) Version() string { return "1.0" }

func (d *StabilityDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	var obs []quality.Observation
	m := input.Measurements
	if m == nil || len(m.FrameMeasurements) < 2 {
		return obs, nil
	}
	mp := input.Profile.MotionPolicy
	maxAnchorJitter := mp.MaxAnchorJitter
	if maxAnchorJitter <= 0 {
		maxAnchorJitter = detectorParam(input.Profile, d.Key(), "maxAnchorJitter", 0.02)
	}
	maxScaleJitter := mp.MaxScaleJitter
	if maxScaleJitter <= 0 {
		maxScaleJitter = detectorParam(input.Profile, d.Key(), "maxScaleJitter", 0.1)
	}
	space := resolveCoordSpace(m)
	frames := m.FrameMeasurements
	for i := 0; i < len(frames)-1; i++ {
		a := frames[i]
		b := frames[i+1]
		if !mp.AllowHorizontalMotion {
			dx := math.Abs(b.AnchorX - a.AnchorX)
			if !space.normalized && space.widthBound > 0 {
				dx = dx / space.widthBound
			}
			if dx > maxAnchorJitter {
				obs = append(obs, quality.Observation{
					DetectorKey:     d.Key(),
					DetectorVersion: d.Version(),
					FrameIndex:      b.FrameIndex,
					FramePairFrom:   a.FrameIndex,
					FramePairTo:     b.FrameIndex,
					MetricName:      quality.RuleAnchorJitter,
					Value:           dx,
					Confidence:      0.85,
					Details: map[string]float64{
						"axis":            0,
						"delta":           dx,
						"maxAnchorJitter": maxAnchorJitter,
					},
				})
			}
		}
		if !mp.AllowVerticalMotion {
			dy := math.Abs(b.AnchorY - a.AnchorY)
			if !space.normalized && space.heightBound > 0 {
				dy = dy / space.heightBound
			}
			if dy > maxAnchorJitter {
				obs = append(obs, quality.Observation{
					DetectorKey:     d.Key(),
					DetectorVersion: d.Version(),
					FrameIndex:      b.FrameIndex,
					FramePairFrom:   a.FrameIndex,
					FramePairTo:     b.FrameIndex,
					MetricName:      quality.RuleAnchorJitter,
					Value:           dy,
					Confidence:      0.85,
					Details: map[string]float64{
						"axis":            1,
						"delta":           dy,
						"maxAnchorJitter": maxAnchorJitter,
					},
				})
			}
		}
		if !mp.AllowScaleChange {
			if a.SubjectBoxHeight > 0 {
				dh := math.Abs(b.SubjectBoxHeight-a.SubjectBoxHeight) / a.SubjectBoxHeight
				if dh > maxScaleJitter {
					obs = append(obs, quality.Observation{
						DetectorKey:     d.Key(),
						DetectorVersion: d.Version(),
						FrameIndex:      b.FrameIndex,
						FramePairFrom:   a.FrameIndex,
						FramePairTo:     b.FrameIndex,
						MetricName:      quality.RuleScaleJitter,
						Value:           dh,
						Confidence:      0.85,
						Details: map[string]float64{
							"axis":           1,
							"delta":          dh,
							"maxScaleJitter": maxScaleJitter,
						},
					})
				}
			}
			if a.SubjectBoxWidth > 0 {
				dw := math.Abs(b.SubjectBoxWidth-a.SubjectBoxWidth) / a.SubjectBoxWidth
				if dw > maxScaleJitter {
					obs = append(obs, quality.Observation{
						DetectorKey:     d.Key(),
						DetectorVersion: d.Version(),
						FrameIndex:      b.FrameIndex,
						FramePairFrom:   a.FrameIndex,
						FramePairTo:     b.FrameIndex,
						MetricName:      quality.RuleScaleJitter,
						Value:           dw,
						Confidence:      0.85,
						Details: map[string]float64{
							"axis":           0,
							"delta":          dw,
							"maxScaleJitter": maxScaleJitter,
						},
					})
				}
			}
		}
	}
	return obs, nil
}
