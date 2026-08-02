// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
)

type DefaultRuleEvaluator struct{}

func NewRuleEvaluator() *DefaultRuleEvaluator {
	return &DefaultRuleEvaluator{}
}

func (e *DefaultRuleEvaluator) Evaluate(ctx context.Context, observations []Observation, profile QualityProfileSnapshot) ([]QualityFinding, error) {
	findings := make([]QualityFinding, 0, len(observations))

	for _, obs := range observations {
		ruleCode := obs.MetricName
		if ruleCode == "" {
			continue
		}

		ruleCfg, hasRule := profile.GetRuleConfig(ruleCode)
		if !hasRule {
			continue
		}

		severity, hardGate := e.resolveSeverityAndGate(obs, ruleCfg, profile)
		if severity == "" {
			continue
		}

		dimension := e.ruleToDimension(ruleCode)

		finding := QualityFinding{
			ID:              uuid.NewString(),
			RuleCode:        ruleCode,
			RuleVersion:     ruleCfg.RuleVersion,
			Dimension:       dimension,
			Severity:        severity,
			MessageKey:      ruleCode,
			Message:         e.buildMessage(ruleCode, obs, ruleCfg),
			Confidence:      obs.Confidence,
			HardGate:        hardGate,
			MetricName:      obs.MetricName,
			Comparison:      ruleCfg.Comparison,
			SuggestedAction: e.buildSuggestion(ruleCode, obs),
			EvidenceRef:     fmt.Sprintf("detector=%s:frame=%d", obs.DetectorKey, obs.FrameIndex),
		}

		if obs.FrameIndex >= 0 {
			finding.FrameIndexes = []int{obs.FrameIndex}
		}
		if obs.FramePairFrom != obs.FramePairTo && (obs.FramePairFrom > 0 || obs.FramePairTo > 0) {
			finding.FramePairs = []FramePairRef{{From: obs.FramePairFrom, To: obs.FramePairTo}}
		}

		finding.ObservedValue = &obs.Value

		switch ruleCfg.Comparison {
		case ">":
			if ruleCfg.WarningThreshold != nil {
				finding.ThresholdValue = ruleCfg.WarningThreshold
			}
		case "<":
			if ruleCfg.WarningThreshold != nil {
				finding.ThresholdValue = ruleCfg.WarningThreshold
			}
		case ">=":
			if ruleCfg.WarningThreshold != nil {
				finding.ThresholdValue = ruleCfg.WarningThreshold
			}
		default:
			if ruleCfg.RejectThreshold != nil {
				finding.ThresholdValue = ruleCfg.RejectThreshold
			} else if ruleCfg.WarningThreshold != nil {
				finding.ThresholdValue = ruleCfg.WarningThreshold
			}
		}

		findings = append(findings, finding)
	}

	sort.SliceStable(findings, func(i, j int) bool {
		si := severitySortOrder(findings[i].Severity)
		sj := severitySortOrder(findings[j].Severity)
		if si != sj {
			return si < sj
		}
		if findings[i].Dimension != findings[j].Dimension {
			return findings[i].Dimension < findings[j].Dimension
		}
		fi := firstFrameIndex(findings[i].FrameIndexes, findings[i].FramePairs)
		fj := firstFrameIndex(findings[j].FrameIndexes, findings[j].FramePairs)
		if fi != fj {
			return fi < fj
		}
		return findings[i].RuleCode < findings[j].RuleCode
	})

	return findings, nil
}

func (e *DefaultRuleEvaluator) resolveSeverityAndGate(obs Observation, ruleCfg RuleConfig, profile QualityProfileSnapshot) (Severity, bool) {
	if ruleCfg.HardGate || profile.IsHardGate(obs.MetricName) {
		return ruleCfg.Severity, true
	}

	val := obs.Value
	warn := ruleCfg.WarningThreshold
	review := ruleCfg.ReviewThreshold
	reject := ruleCfg.RejectThreshold

	switch ruleCfg.Comparison {
	case ">", ">=":
		if reject != nil && val >= *reject {
			return SeverityError, false
		}
		if review != nil && val >= *review {
			return SeverityReview, false
		}
		if warn != nil && val >= *warn {
			return SeverityWarning, false
		}
	case "<":
		if reject != nil && val <= *reject {
			return SeverityError, false
		}
		if review != nil && val <= *review {
			return SeverityReview, false
		}
		if warn != nil && val <= *warn {
			return SeverityWarning, false
		}
	case "==":
		return ruleCfg.Severity, ruleCfg.HardGate
	case "missing", "failure", "legacy", "policy", "clip", "gap":
		return ruleCfg.Severity, ruleCfg.HardGate
	default:
		if reject != nil && val >= *reject {
			return SeverityError, false
		}
		if review != nil && val >= *review {
			return SeverityReview, false
		}
		if warn != nil && val >= *warn {
			return SeverityWarning, false
		}
	}

	return "", false
}

func (e *DefaultRuleEvaluator) ruleToDimension(ruleCode string) string {
	switch ruleCode {
	case RuleFileMissing, RuleFileUndecodable, RuleFileHashMismatch,
		RuleFrameCountMismatch, RuleFrameIndexGap, RuleFrameDimensionMismatch:
		return DimensionIntegrity
	case RuleAlphaAllTransparent, RuleSubjectEmpty, RuleSubjectTooSmall,
		RuleSubjectTooLarge, RuleSubjectFragmented, RuleSubjectClipped:
		return DimensionSubjectIntegrity
	case RuleAlphaPolicyViolation, RuleBackgroundResidueComponent, RuleAlphaHalo:
		return DimensionBackgroundCleanliness
	case RuleUnexpectedEdgeContact:
		return DimensionSubjectIntegrity
	case RuleAnchorJitter, RuleScaleJitter:
		return DimensionAnchorStability
	case RuleIdentityDrift:
		return DimensionIdentityConsistency
	case RuleMotionJump, RuleMotionDirectionReversal,
		RuleExactDuplicateFrame, RulePerceptualDuplicateFrame, RuleFrozenSequence:
		return DimensionMotionContinuity
	case RuleLoopDiscontinuity, RuleLoopVelocityDiscontinuity:
		return DimensionLoopContinuity
	case RuleColorFlicker:
		return DimensionVisualConsistency
	case RuleLowEvaluationConfidence, RuleMissingMeasurement, RuleDetectorFailure:
		return DimensionEvaluationConfidence
	case RuleLegacyFlagImported:
		return DimensionEvaluationConfidence
	default:
		return "unknown"
	}
}

func (e *DefaultRuleEvaluator) buildMessage(ruleCode string, obs Observation, ruleCfg RuleConfig) string {
	return fmt.Sprintf("%s: %.4f (threshold: %s)", ruleCode, obs.Value, ruleCfg.Comparison)
}

func (e *DefaultRuleEvaluator) buildSuggestion(ruleCode string, obs Observation) string {
	switch ruleCode {
	case RuleFileMissing:
		return fmt.Sprintf("帧 %d 文件缺失，请重新处理该帧", obs.FrameIndex)
	case RuleFileUndecodable:
		return fmt.Sprintf("帧 %d 无法解码，请检查文件完整性", obs.FrameIndex)
	case RuleFileHashMismatch:
		return fmt.Sprintf("帧 %d 哈希不匹配，文件可能被篡改", obs.FrameIndex)
	case RuleAlphaAllTransparent:
		return fmt.Sprintf("帧 %d 完全透明，请重新生成", obs.FrameIndex)
	case RuleSubjectEmpty:
		return fmt.Sprintf("帧 %d 未检测到主体，请重新生成", obs.FrameIndex)
	case RuleSubjectClipped:
		return fmt.Sprintf("帧 %d 主体被裁切，请调整画布或重新生成", obs.FrameIndex)
	case RuleAnchorJitter:
		return fmt.Sprintf("帧 %d→%d 锚点抖动异常，请在动作编辑器中重新对齐", obs.FramePairFrom, obs.FramePairTo)
	case RuleScaleJitter:
		return fmt.Sprintf("帧 %d→%d 尺度异常变化，请检查缩放参数", obs.FramePairFrom, obs.FramePairTo)
	case RuleMotionJump:
		return fmt.Sprintf("帧 %d→%d 运动突变，请重新生成该帧对", obs.FramePairFrom, obs.FramePairTo)
	case RuleLoopDiscontinuity:
		return "循环头尾不连续，请检查首尾帧衔接"
	case RuleFrozenSequence:
		return fmt.Sprintf("帧 %d 起连续冻结，请检查动画是否正常", obs.FrameIndex)
	case RuleIdentityDrift:
		return fmt.Sprintf("帧 %d 角色视觉特征异常变化，请检查角色一致性", obs.FrameIndex)
	case RuleColorFlicker:
		return fmt.Sprintf("帧 %d→%d 色彩闪烁，请检查颜色一致性", obs.FramePairFrom, obs.FramePairTo)
	case RuleBackgroundResidueComponent:
		return fmt.Sprintf("帧 %d 背景残留，请重新抠图", obs.FrameIndex)
	default:
		return fmt.Sprintf("帧 %d 存在 %s 问题", obs.FrameIndex, ruleCode)
	}
}

func isValidScore(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
