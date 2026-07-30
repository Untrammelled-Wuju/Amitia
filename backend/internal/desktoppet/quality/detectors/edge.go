// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type EdgeDetector struct{}

var _ quality.Detector = (*EdgeDetector)(nil)

func NewEdgeDetector() *EdgeDetector {
	return &EdgeDetector{}
}

func (d *EdgeDetector) Key() string     { return "edge" }
func (d *EdgeDetector) Version() string { return "1.0" }

func edgeAllowed(allowed []string, side string) bool {
	for _, s := range allowed {
		if s == side {
			return true
		}
	}
	return false
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (d *EdgeDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	var obs []quality.Observation
	m := input.Measurements
	if m == nil || len(m.FrameMeasurements) == 0 {
		return obs, nil
	}
	space := resolveCoordSpace(m)
	if space.widthBound <= 0 || space.heightBound <= 0 || space.marginX <= 0 || space.marginY <= 0 {
		return obs, nil
	}
	allowed := input.Profile.MotionPolicy.AllowedEdges
	for _, f := range m.FrameMeasurements {
		leftClip := f.SubjectBoxX < 0
		topClip := f.SubjectBoxY < 0
		rightClip := f.SubjectBoxX+f.SubjectBoxWidth > space.widthBound
		bottomClip := f.SubjectBoxY+f.SubjectBoxHeight > space.heightBound
		if leftClip || topClip || rightClip || bottomClip {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleSubjectClipped,
				Value:           1,
				Confidence:      0.9,
				Details: map[string]float64{
					"left":   boolToFloat(leftClip),
					"top":    boolToFloat(topClip),
					"right":  boolToFloat(rightClip),
					"bottom": boolToFloat(bottomClip),
				},
			})
		}
		leftDepth := 0.0
		if f.SubjectBoxX < space.marginX {
			leftDepth = (space.marginX - f.SubjectBoxX) / space.marginX
		}
		topDepth := 0.0
		if f.SubjectBoxY < space.marginY {
			topDepth = (space.marginY - f.SubjectBoxY) / space.marginY
		}
		rightDepth := 0.0
		if f.SubjectBoxX+f.SubjectBoxWidth > space.widthBound-space.marginX {
			rightDepth = (f.SubjectBoxX + f.SubjectBoxWidth - (space.widthBound - space.marginX)) / space.marginX
		}
		bottomDepth := 0.0
		if f.SubjectBoxY+f.SubjectBoxHeight > space.heightBound-space.marginY {
			bottomDepth = (f.SubjectBoxY + f.SubjectBoxHeight - (space.heightBound - space.marginY)) / space.marginY
		}
		details := map[string]float64{}
		maxDepth := 0.0
		unexpectedCount := 0
		if leftDepth > 0 && !leftClip {
			details["leftDepth"] = clamp01(leftDepth)
			if leftDepth > maxDepth {
				maxDepth = leftDepth
			}
			if !edgeAllowed(allowed, "left") {
				unexpectedCount++
			}
		}
		if topDepth > 0 && !topClip {
			details["topDepth"] = clamp01(topDepth)
			if topDepth > maxDepth {
				maxDepth = topDepth
			}
			if !edgeAllowed(allowed, "top") {
				unexpectedCount++
			}
		}
		if rightDepth > 0 && !rightClip {
			details["rightDepth"] = clamp01(rightDepth)
			if rightDepth > maxDepth {
				maxDepth = rightDepth
			}
			if !edgeAllowed(allowed, "right") {
				unexpectedCount++
			}
		}
		if bottomDepth > 0 && !bottomClip {
			details["bottomDepth"] = clamp01(bottomDepth)
			if bottomDepth > maxDepth {
				maxDepth = bottomDepth
			}
			if !edgeAllowed(allowed, "bottom") {
				unexpectedCount++
			}
		}
		if unexpectedCount > 0 {
			details["unexpectedCount"] = float64(unexpectedCount)
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleUnexpectedEdgeContact,
				Value:           clamp01(maxDepth),
				Confidence:      0.8,
				Details:         details,
			})
		}
	}
	return obs, nil
}
