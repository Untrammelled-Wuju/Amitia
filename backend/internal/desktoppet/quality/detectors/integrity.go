// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/quality"
)

func ruleThreshold(profile quality.QualityProfileSnapshot, ruleCode string, def float64) float64 {
	if cfg, ok := profile.GetRuleConfig(ruleCode); ok && cfg.WarningThreshold != nil {
		return *cfg.WarningThreshold
	}
	return def
}

func detectorParam(profile quality.QualityProfileSnapshot, detectorKey, param string, def float64) float64 {
	if cfg, ok := profile.GetDetectorConfig(detectorKey); ok {
		if v, ok := cfg.Parameters[param]; ok {
			return v
		}
	}
	return def
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

type frameCoordSpace struct {
	widthBound  float64
	heightBound float64
	marginX     float64
	marginY     float64
	normalized  bool
}

func resolveCoordSpace(m *quality.ActionMeasurementSet) frameCoordSpace {
	normalized := true
	for _, f := range m.FrameMeasurements {
		if f.SubjectBoxWidth > 1.5 || f.SubjectBoxHeight > 1.5 ||
			f.SubjectBoxX > 1.5 || f.SubjectBoxY > 1.5 {
			normalized = false
			break
		}
	}
	if normalized {
		return frameCoordSpace{
			widthBound:  1.0,
			heightBound: 1.0,
			marginX:     0.02,
			marginY:     0.02,
			normalized:  true,
		}
	}
	cw := float64(m.CanvasWidth)
	ch := float64(m.CanvasHeight)
	return frameCoordSpace{
		widthBound:  cw,
		heightBound: ch,
		marginX:     cw * 0.02,
		marginY:     ch * 0.02,
		normalized:  false,
	}
}

type IntegrityDetector struct{}

var _ quality.Detector = (*IntegrityDetector)(nil)

func NewIntegrityDetector() *IntegrityDetector {
	return &IntegrityDetector{}
}

func (d *IntegrityDetector) Key() string     { return "integrity" }
func (d *IntegrityDetector) Version() string { return "1.0" }

func (d *IntegrityDetector) Detect(ctx context.Context, input quality.DetectorInput) ([]quality.Observation, error) {
	var obs []quality.Observation
	m := input.Measurements
	if m == nil {
		return obs, nil
	}
	for _, f := range m.FrameMeasurements {
		if !f.FileExists {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleFileMissing,
				Value:           0,
				Confidence:      1.0,
			})
			continue
		}
		if !f.Decodable {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleFileUndecodable,
				Value:           0,
				Confidence:      1.0,
			})
		}
		if f.PixelHash == "" {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleFileHashMismatch,
				Value:           0,
				Confidence:      0.8,
			})
		}
		if m.CanvasWidth > 0 && m.CanvasHeight > 0 && (f.Width != m.CanvasWidth || f.Height != m.CanvasHeight) {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleFrameDimensionMismatch,
				Value:           1,
				Confidence:      1.0,
				Details: map[string]float64{
					"frameWidth":   float64(f.Width),
					"frameHeight":  float64(f.Height),
					"canvasWidth":  float64(m.CanvasWidth),
					"canvasHeight": float64(m.CanvasHeight),
				},
			})
		}
	}
	if m.FrameCount > 0 && m.FrameCount != len(m.FrameMeasurements) {
		obs = append(obs, quality.Observation{
			DetectorKey:     d.Key(),
			DetectorVersion: d.Version(),
			FrameIndex:      -1,
			MetricName:      quality.RuleFrameCountMismatch,
			Value:           float64(len(m.FrameMeasurements)),
			Confidence:      1.0,
			Details: map[string]float64{
				"declared": float64(m.FrameCount),
				"actual":   float64(len(m.FrameMeasurements)),
			},
		})
	}
	seen := make(map[int]bool, len(m.FrameMeasurements))
	minIndex := -1
	maxIndex := -1
	for _, f := range m.FrameMeasurements {
		if seen[f.FrameIndex] {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      f.FrameIndex,
				MetricName:      quality.RuleFrameIndexGap,
				Value:           1,
				Confidence:      1.0,
				Details: map[string]float64{
					"duplicate": 1,
				},
			})
		}
		seen[f.FrameIndex] = true
		if minIndex == -1 || f.FrameIndex < minIndex {
			minIndex = f.FrameIndex
		}
		if f.FrameIndex > maxIndex {
			maxIndex = f.FrameIndex
		}
	}
	if minIndex >= 0 {
		gaps := 0
		for i := minIndex; i <= maxIndex; i++ {
			if !seen[i] {
				gaps++
			}
		}
		if gaps > 0 {
			obs = append(obs, quality.Observation{
				DetectorKey:     d.Key(),
				DetectorVersion: d.Version(),
				FrameIndex:      -1,
				MetricName:      quality.RuleFrameIndexGap,
				Value:           float64(gaps),
				Confidence:      1.0,
				Details: map[string]float64{
					"gapCount": float64(gaps),
				},
			})
		}
	}
	return obs, nil
}
