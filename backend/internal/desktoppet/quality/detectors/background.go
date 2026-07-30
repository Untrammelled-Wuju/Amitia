// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

type BackgroundDetector struct{}

var _ quality.Detector = (*BackgroundDetector)(nil)

func NewBackgroundDetector() *BackgroundDetector {
	return &BackgroundDetector{}
}

func (d *BackgroundDetector) Key() string     { return "background" }
func (d *BackgroundDetector) Version() string { return "1.0" }

func (d *BackgroundDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	var obs []quality.Observation
	m := input.Measurements
	if m == nil {
		return obs, nil
	}
	policy := input.Profile.BackgroundPolicy
	if policy == "" {
		policy = quality.BackgroundPolicyRemoveBackground
	}
	residueThreshold := ruleThreshold(input.Profile, quality.RuleBackgroundResidueComponent, 0.05)
	borderThreshold := detectorParam(input.Profile, d.Key(), "borderResidueThreshold", 0.02)
	haloThreshold := ruleThreshold(input.Profile, quality.RuleAlphaHalo, 0.1)
	checkOpaque := policy == quality.BackgroundPolicyRemoveBackground
	for _, f := range m.FrameMeasurements {
		if checkOpaque && f.OpaqueCoverage > residueThreshold {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleBackgroundResidueComponent,
				Value:           f.OpaqueCoverage,
				Confidence:      0.85,
				Details: map[string]float64{
					"opaqueCoverage":   f.OpaqueCoverage,
					"residueThreshold": residueThreshold,
					"kind":             1,
				},
			})
		}
		if checkOpaque && f.BorderForegroundCoverage > borderThreshold {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleBackgroundResidueComponent,
				Value:           f.BorderForegroundCoverage,
				Confidence:      0.8,
				Details: map[string]float64{
					"borderForegroundCoverage": f.BorderForegroundCoverage,
					"borderThreshold":          borderThreshold,
					"kind":                     2,
				},
			})
		}
		if f.SemiTransparentCoverage > haloThreshold {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleAlphaHalo,
				Value:           f.SemiTransparentCoverage,
				Confidence:      0.75,
				Details: map[string]float64{
					"semiTransparentCoverage": f.SemiTransparentCoverage,
					"haloThreshold":           haloThreshold,
				},
			})
		}
		if policy == quality.BackgroundPolicyKeepAlpha && f.OpaqueCoverage > residueThreshold {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleAlphaPolicyViolation,
				Value:           f.OpaqueCoverage,
				Confidence:      0.8,
				Details: map[string]float64{
					"opaqueCoverage":   f.OpaqueCoverage,
					"residueThreshold": residueThreshold,
				},
			})
		}
	}
	return obs, nil
}
