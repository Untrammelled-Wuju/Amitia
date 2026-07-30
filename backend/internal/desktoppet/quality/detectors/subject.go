// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type SubjectDetector struct{}

var _ quality.Detector = (*SubjectDetector)(nil)

func NewSubjectDetector() *SubjectDetector {
	return &SubjectDetector{}
}

func (d *SubjectDetector) Key() string     { return "subject" }
func (d *SubjectDetector) Version() string { return "1.0" }

func (d *SubjectDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	var obs []quality.Observation
	m := input.Measurements
	if m == nil {
		return obs, nil
	}
	smallThreshold := ruleThreshold(input.Profile, quality.RuleSubjectTooSmall, 0.01)
	largeThreshold := ruleThreshold(input.Profile, quality.RuleSubjectTooLarge, 0.9)
	fragThreshold := ruleThreshold(input.Profile, quality.RuleSubjectFragmented, 0.15)
	space := resolveCoordSpace(m)
	canvasArea := space.widthBound * space.heightBound
	for _, f := range m.FrameMeasurements {
		if f.ForegroundCoverage <= 0 {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleAlphaAllTransparent,
				Value:           0,
				Confidence:      1.0,
				Details: map[string]float64{
					"foregroundCoverage": f.ForegroundCoverage,
				},
			})
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleSubjectEmpty,
				Value:           0,
				Confidence:      1.0,
			})
			continue
		}
		if f.MaskArea < smallThreshold {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleSubjectTooSmall,
				Value:           f.MaskArea,
				Confidence:      0.9,
				Details: map[string]float64{
					"maskArea":       f.MaskArea,
					"smallThreshold": smallThreshold,
				},
			})
		}
		if f.MaskArea > largeThreshold {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleSubjectTooLarge,
				Value:           f.MaskArea,
				Confidence:      0.9,
				Details: map[string]float64{
					"maskArea":       f.MaskArea,
					"largeThreshold": largeThreshold,
				},
			})
		}
		boxArea := f.SubjectBoxWidth * f.SubjectBoxHeight
		if canvasArea > 0 && boxArea > 0 {
			boxCoverage := boxArea / canvasArea
			fillRatio := f.MaskArea / boxCoverage
			if boxCoverage > smallThreshold && fillRatio < fragThreshold {
				obs = append(obs, quality.Observation{
					DetectorKey:     d.Key(),
					DetectorVersion: d.Version(),
					FrameIndex:      f.FrameIndex,
					MetricName:      quality.RuleSubjectFragmented,
					Value:           fillRatio,
					Confidence:      0.7,
					Details: map[string]float64{
						"maskArea":      f.MaskArea,
						"boxCoverage":   boxCoverage,
						"fillRatio":     fillRatio,
						"fragThreshold": fragThreshold,
					},
				})
			}
		}
		if space.widthBound > 0 && space.heightBound > 0 {
			clipped := f.SubjectBoxX < 0 || f.SubjectBoxY < 0 ||
				f.SubjectBoxX+f.SubjectBoxWidth > space.widthBound ||
				f.SubjectBoxY+f.SubjectBoxHeight > space.heightBound
			if clipped {
				obs = append(obs, quality.Observation{
					DetectorKey:     d.Key(),
					DetectorVersion: d.Version(),
					FrameIndex:      f.FrameIndex,
					MetricName:      quality.RuleSubjectClipped,
					Value:           1,
					Confidence:      0.9,
					Details: map[string]float64{
						"boxX":        f.SubjectBoxX,
						"boxY":        f.SubjectBoxY,
						"boxWidth":    f.SubjectBoxWidth,
						"boxHeight":   f.SubjectBoxHeight,
						"widthBound":  space.widthBound,
						"heightBound": space.heightBound,
					},
				})
			}
		}
	}
	return obs, nil
}
