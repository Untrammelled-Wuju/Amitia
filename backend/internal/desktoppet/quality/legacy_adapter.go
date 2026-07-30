// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"strings"

	"github.com/google/uuid"
)

type LegacyAdapter struct{}

func NewLegacyAdapter() *LegacyAdapter {
	return &LegacyAdapter{}
}

func (a *LegacyAdapter) ConvertLegacyFlags(flags []string) []QualityFinding {
	findings := make([]QualityFinding, 0, len(flags))
	for _, raw := range flags {
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			flag := strings.TrimSpace(part)
			if flag == "" {
				continue
			}
			finding, ok := a.convertFlag(flag)
			if !ok {
				continue
			}
			findings = append(findings, finding)
		}
	}
	return findings
}

func (a *LegacyAdapter) convertFlag(flag string) (QualityFinding, bool) {
	var ruleCode string
	var severity Severity
	var confidence float64

	switch flag {
	case "SUBJECT_TOO_SMALL":
		ruleCode = RuleSubjectTooSmall
		severity = SeverityWarning
		confidence = 0.3
	case "SUBJECT_TOO_LARGE":
		ruleCode = RuleSubjectTooLarge
		severity = SeverityWarning
		confidence = 0.3
	case "BACKGROUND_RESIDUE":
		ruleCode = RuleBackgroundResidueComponent
		severity = SeverityWarning
		confidence = 0.2
	case "EMPTY_FRAME":
		ruleCode = RuleAlphaAllTransparent
		severity = SeverityCritical
		confidence = 0.5
	case "DUPLICATE_FRAME":
		ruleCode = RuleExactDuplicateFrame
		severity = SeverityInfo
		confidence = 0.3
	case "ALPHA_INVALID":
		ruleCode = RuleAlphaPolicyViolation
		severity = SeverityWarning
		confidence = 0.3
	case "SOURCE_MISSING":
		ruleCode = RuleFileMissing
		severity = SeverityCritical
		confidence = 0.5
	case "LOOP_DISCONTINUITY":
		ruleCode = RuleLoopDiscontinuity
		severity = SeverityWarning
		confidence = 0.3
	case "SCALE_DRIFT":
		ruleCode = RuleScaleJitter
		severity = SeverityWarning
		confidence = 0.3
	case "ANCHOR_DRIFT":
		ruleCode = RuleAnchorJitter
		severity = SeverityWarning
		confidence = 0.3
	default:
		return QualityFinding{}, false
	}

	dimension := NewRuleEvaluator().ruleToDimension(ruleCode)

	return QualityFinding{
		ID:          uuid.NewString(),
		RuleCode:    ruleCode,
		RuleVersion: 1,
		Dimension:   dimension,
		Severity:    severity,
		MessageKey:  "LEGACY_FLAG_IMPORTED",
		Confidence:  confidence,
		EvidenceRef: "legacy_quality_flags",
	}, true
}

func (a *LegacyAdapter) ConvertLegacyLevel(level string) ContentVerdict {
	switch level {
	case "normal":
		return VerdictAcceptedWithWarning
	case "warning":
		return VerdictNeedsReview
	case "failed":
		return VerdictRejected
	default:
		return VerdictNeedsReview
	}
}
