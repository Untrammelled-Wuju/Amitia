// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"math"
	"sort"
)

type DefaultScorer struct{}

func NewScorer() *DefaultScorer {
	return &DefaultScorer{}
}

func (s *DefaultScorer) Score(findings []QualityFinding, observations []Observation, profile QualityProfileSnapshot) ([]DimensionScore, float64, float64, error) {
	dimensionFindings := make(map[string][]QualityFinding)
	for _, f := range findings {
		dimensionFindings[f.Dimension] = append(dimensionFindings[f.Dimension], f)
	}

	dimensionObs := make(map[string][]Observation)
	for _, obs := range observations {
		dim := s.metricToDimension(obs.MetricName, profile)
		if dim != "" {
			dimensionObs[dim] = append(dimensionObs[dim], obs)
		}
	}

	allDimensions := []string{
		DimensionIntegrity,
		DimensionSubjectIntegrity,
		DimensionBackgroundCleanliness,
		DimensionAnchorStability,
		DimensionIdentityConsistency,
		DimensionMotionContinuity,
		DimensionLoopContinuity,
		DimensionVisualConsistency,
		DimensionEvaluationConfidence,
	}

	scores := make([]DimensionScore, 0, len(allDimensions))
	var totalWeight float64
	var weightedScore float64
	var weightedConfidence float64
	var confidenceWeight float64

	for _, dim := range allDimensions {
		dimCfg, hasCfg := profile.GetDimensionConfig(dim)
		if !hasCfg {
			dimCfg = DimensionConfig{Weight: 1.0, PassScore: 75, MinConfidence: 0.5}
		}

		dimFindings := dimensionFindings[dim]
		dimObs := dimensionObs[dim]

		applicability, score, confidence := s.scoreDimension(dim, dimFindings, dimObs, dimCfg, profile)

		findingIDs := make([]string, 0, len(dimFindings))
		for _, f := range dimFindings {
			findingIDs = append(findingIDs, f.ID)
		}

		ds := DimensionScore{
			Dimension:     dim,
			Applicability: applicability,
			Confidence:    confidence,
			Weight:        dimCfg.Weight,
			FindingIDs:    findingIDs,
		}
		if score != nil {
			ds.Score = score
		}

		scores = append(scores, ds)

		if applicability == ApplicabilityApplicable && score != nil && dimCfg.Weight > 0 {
			totalWeight += dimCfg.Weight
			weightedScore += *score * dimCfg.Weight
			confidenceWeight += dimCfg.Weight
			weightedConfidence += confidence * dimCfg.Weight
		}
	}

	var overallScore float64
	var overallConfidence float64

	if totalWeight > 0 {
		overallScore = weightedScore / totalWeight
	} else {
		overallScore = 100
	}

	if confidenceWeight > 0 {
		overallConfidence = weightedConfidence / confidenceWeight
	} else {
		overallConfidence = 1.0
	}

	if !isValidScore(overallScore) {
		overallScore = 0
	}
	if !isValidScore(overallConfidence) {
		overallConfidence = 0
	}

	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].Dimension < scores[j].Dimension
	})

	return scores, overallScore, overallConfidence, nil
}

func (s *DefaultScorer) scoreDimension(dimension string, findings []QualityFinding, observations []Observation, dimCfg DimensionConfig, profile QualityProfileSnapshot) (Applicability, *float64, float64) {
	hasHardGate := false
	hasCritical := false
	hasError := false
	totalPenalty := 0.0
	maxConfidence := 0.0

	for _, f := range findings {
		if f.HardGate {
			hasHardGate = true
		}
		switch f.Severity {
		case SeverityCritical:
			hasCritical = true
		case SeverityError:
			hasError = true
		}
		if f.Confidence > maxConfidence {
			maxConfidence = f.Confidence
		}

		ruleCfg, _ := profile.GetRuleConfig(f.RuleCode)
		penalty := ruleCfg.MaxPenalty
		if penalty <= 0 {
			penalty = 20
		}
		totalPenalty += penalty
	}

	if len(observations) == 0 && len(findings) == 0 {
		return ApplicabilityNotApplicable, nil, 1.0
	}

	if dimension == DimensionIntegrity {
		if hasHardGate || hasCritical {
			score := 0.0
			return ApplicabilityApplicable, &score, maxConfidence
		}
		if len(findings) == 0 {
			score := 100.0
			return ApplicabilityApplicable, &score, 1.0
		}
	}

	if dimension == DimensionEvaluationConfidence {
		if len(findings) == 0 {
			score := 100.0
			return ApplicabilityApplicable, &score, 1.0
		}
	}

	var score float64
	if hasHardGate || hasCritical {
		score = 0.0
	} else if hasError {
		score = 25.0
	} else {
		penaltyRatio := math.Min(totalPenalty/100.0, 1.0)
		score = 100.0 * (1.0 - penaltyRatio)
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	confidence := maxConfidence
	if confidence == 0 && len(observations) > 0 {
		confidence = 0.5
	}
	if confidence == 0 {
		confidence = 1.0
	}

	return ApplicabilityApplicable, &score, confidence
}

func (s *DefaultScorer) metricToDimension(metricName string, profile QualityProfileSnapshot) string {
	evaluator := &DefaultRuleEvaluator{}
	return evaluator.ruleToDimension(metricName)
}

type DefaultVerdictResolver struct{}

func NewVerdictResolver() *DefaultVerdictResolver {
	return &DefaultVerdictResolver{}
}

func (r *DefaultVerdictResolver) Resolve(findings []QualityFinding, scores []DimensionScore, overallScore float64, overallConfidence float64, profile QualityProfileSnapshot) ContentVerdict {
	for _, f := range findings {
		if f.HardGate {
			return VerdictRejected
		}
	}

	hasCritical := false
	hasError := false
	hasReview := false
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			hasCritical = true
		case SeverityError:
			hasError = true
		case SeverityReview:
			hasReview = true
		}
	}

	if hasCritical {
		return VerdictRejected
	}

	for _, ds := range scores {
		if ds.Applicability != ApplicabilityApplicable {
			continue
		}
		dimCfg, hasCfg := profile.GetDimensionConfig(ds.Dimension)
		if !hasCfg {
			continue
		}
		if dimCfg.CriticalDimension && ds.Score != nil && *ds.Score < dimCfg.PassScore {
			return VerdictRejected
		}
	}

	if hasError {
		return VerdictNeedsReview
	}

	if overallConfidence < 0.55 {
		return VerdictNeedsReview
	}

	for _, ds := range scores {
		if ds.Applicability != ApplicabilityApplicable {
			continue
		}
		if ds.Score != nil && *ds.Score < 75 {
			return VerdictNeedsReview
		}
	}

	if hasReview {
		return VerdictAcceptedWithWarning
	}

	if overallScore < 85 {
		return VerdictAcceptedWithWarning
	}

	return VerdictAccepted
}
