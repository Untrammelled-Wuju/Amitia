package decision

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	BehaviorFormulaVersionV1   = "behavior-scoring-v1"
	BehaviorFormulaVersionV2   = "behavior-scoring-v2"
	BehaviorParameterVersionV1 = "behavior-scoring-params-v1"
)

type BehaviorScoringOptions struct {
	BaseWeight           float64
	PersonalityWeight    float64
	NeedWeight           float64
	RelationshipWeight   float64
	AffectWeight         float64
	UserPreferenceWeight float64
	RiskWeight           float64
	RepeatPenaltyWeight  float64
	FatiguePenaltyWeight float64
}

type CandidateScoringContext struct {
	Goals              []Goal
	Intentions         []Intention
	Psyche             PsycheSignalSet
	Relationship       RelationshipSnapshot
	Life               LifeSnapshot
	PersonalityWeights map[BehaviorTag]float64
	UserPreferences    map[string]float64
	History            BehaviorHistory
	Now                time.Time
}

type BehaviorSelectionResult struct {
	Selected     BehaviorCandidate   `json:"selected"`
	Alternatives []BehaviorCandidate `json:"alternatives,omitempty"`
	Blocked      []BehaviorCandidate `json:"blocked,omitempty"`
	Audit        BehaviorAudit       `json:"audit"`
}

type AffectRegulator struct {
	PositiveEmotionWeight float64
	NegativeEmotionWeight float64
	StressWeight          float64
	MaxMultiplier         float64
	MinMultiplier         float64
}

func DefaultAffectRegulator() AffectRegulator {
	return AffectRegulator{
		PositiveEmotionWeight: 0.35,
		NegativeEmotionWeight: 0.40,
		StressWeight:          0.25,
		MaxMultiplier:         1.25,
		MinMultiplier:         0.65,
	}
}

func DefaultBehaviorScoringOptions() BehaviorScoringOptions {
	return BehaviorScoringOptions{
		BaseWeight:           1,
		PersonalityWeight:    1,
		NeedWeight:           1,
		RelationshipWeight:   1,
		AffectWeight:         1,
		UserPreferenceWeight: 0.1,
		RiskWeight:           1,
		RepeatPenaltyWeight:  1,
		FatiguePenaltyWeight: 1,
	}
}

// ValidateScoringOptions 校验评分工子权重合法性
func ValidateScoringOptions(options BehaviorScoringOptions) error {
	weights := map[string]float64{
		"BaseWeight":           options.BaseWeight,
		"PersonalityWeight":    options.PersonalityWeight,
		"NeedWeight":           options.NeedWeight,
		"RelationshipWeight":   options.RelationshipWeight,
		"AffectWeight":         options.AffectWeight,
		"UserPreferenceWeight": options.UserPreferenceWeight,
		"RiskWeight":           options.RiskWeight,
		"RepeatPenaltyWeight":  options.RepeatPenaltyWeight,
		"FatiguePenaltyWeight": options.FatiguePenaltyWeight,
	}
	for name, w := range weights {
		if math.IsNaN(w) || math.IsInf(w, 0) {
			return fmt.Errorf("scoring: weight %s is NaN or Inf", name)
		}
		if w < 0 {
			return fmt.Errorf("scoring: weight %s cannot be negative: %f", name, w)
		}
	}
	return nil
}

// ScoreCandidates 是 B4 以后唯一生产 FinalScore 入口
// 流程: ValidateOptions → ValidateNow → CloneCandidates → ResetScores → ContextSignals → UserPreferenceSignals → CostSignals → ValidateComponents → ComputeFinalScore → Reasons → SetVersion
// 不排序, 不改变数量, 输入多少输出多少
func ScoreCandidates(candidates []BehaviorCandidate, ctx CandidateScoringContext, options BehaviorScoringOptions) ([]BehaviorCandidate, error) {
	if err := ValidateScoringOptions(options); err != nil {
		return nil, err
	}
	if ctx.Now.IsZero() {
		return nil, errors.New("scoring: Now is required")
	}

	result := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := cloneCandidate(candidate)
		next = ApplyCandidateContextSignalsSingle(next, ctx)
		next.UserPreferenceScore = ComputeUserPreferenceScore(next, ctx.UserPreferences)
		next.RepeatPenalty = ComputeRepeatPenalty(ctx.History, candidate.ID, ctx.Now, DefaultRepeatPenaltyConfig())
		next.FatiguePenalty = ComputeFatiguePenalty(ctx.History, candidate.ID, ctx.Now, DefaultFatiguePenaltyConfig())

		if err := validateCandidateScoreComponents(next); err != nil {
			return nil, fmt.Errorf("scoring: candidate %s: %w", next.ID, err)
		}

		next.FinalScore = computeFinalScore(next, options)
		next.Reasons = appendScoringReasons(next.Reasons, next, options)
		next.ScoringVersion = BehaviorFormulaVersionV2

		result = append(result, next)
	}
	return result, nil
}

func cloneCandidate(candidate BehaviorCandidate) BehaviorCandidate {
	next := candidate
	if candidate.Reasons != nil {
		next.Reasons = append([]BehaviorReason{}, candidate.Reasons...)
	}
	if candidate.Constraints != nil {
		next.Constraints = append([]BehaviorConstraint{}, candidate.Constraints...)
	}
	return next
}

func computeFinalScore(candidate BehaviorCandidate, options BehaviorScoringOptions) float64 {
	return round4(
		candidate.BaseScore*options.BaseWeight +
			candidate.PersonalityScore*options.PersonalityWeight +
			candidate.NeedScore*options.NeedWeight +
			candidate.RelationshipScore*options.RelationshipWeight +
			candidate.AffectScore*options.AffectWeight +
			candidate.UserPreferenceScore*options.UserPreferenceWeight -
			candidate.RiskScore*options.RiskWeight -
			candidate.RepeatPenalty*options.RepeatPenaltyWeight -
			candidate.FatiguePenalty*options.FatiguePenaltyWeight,
	)
}

// ApplyCandidateContextSignalsSingle 对所有 Context Signals 应用于单个 Candidate
func ApplyCandidateContextSignalsSingle(candidate BehaviorCandidate, ctx CandidateScoringContext) BehaviorCandidate {
	candidates := ApplyCandidateContextSignals([]BehaviorCandidate{candidate}, CandidateGenerationContext{
		Goals:              ctx.Goals,
		Intentions:         ctx.Intentions,
		Psyche:             ctx.Psyche,
		Relationship:       ctx.Relationship,
		Life:               ctx.Life,
		PersonalityWeights: ctx.PersonalityWeights,
	})
	return candidates[0]
}

func validateCandidateScoreComponents(candidate BehaviorCandidate) error {
	components := map[string]float64{
		"BaseScore":           candidate.BaseScore,
		"PersonalityScore":    candidate.PersonalityScore,
		"NeedScore":           candidate.NeedScore,
		"RelationshipScore":   candidate.RelationshipScore,
		"AffectScore":         candidate.AffectScore,
		"UserPreferenceScore": candidate.UserPreferenceScore,
		"RiskScore":           candidate.RiskScore,
		"RepeatPenalty":       candidate.RepeatPenalty,
		"FatiguePenalty":      candidate.FatiguePenalty,
	}
	for name, v := range components {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("component %s is NaN or Inf", name)
		}
	}
	if candidate.UserPreferenceScore < -1 || candidate.UserPreferenceScore > 1 {
		return fmt.Errorf("UserPreferenceScore %f out of range [-1, 1]", candidate.UserPreferenceScore)
	}
	if candidate.RiskScore < 0 || candidate.RiskScore > 1.2 {
		return fmt.Errorf("RiskScore %f out of expected range", candidate.RiskScore)
	}
	if candidate.RepeatPenalty < 0 {
		return fmt.Errorf("RepeatPenalty cannot be negative")
	}
	if candidate.FatiguePenalty < 0 {
		return fmt.Errorf("FatiguePenalty cannot be negative")
	}
	return nil
}

func appendScoringReasons(reasons []BehaviorReason, candidate BehaviorCandidate, options BehaviorScoringOptions) []BehaviorReason {
	next := stripSource(reasons, "scoring")
	next = append(next,
		BehaviorReason{Source: "scoring", Key: "base", Delta: round4(candidate.BaseScore * options.BaseWeight)},
		BehaviorReason{Source: "scoring", Key: "personality", Delta: round4(candidate.PersonalityScore * options.PersonalityWeight)},
		BehaviorReason{Source: "scoring", Key: "need", Delta: round4(candidate.NeedScore * options.NeedWeight)},
		BehaviorReason{Source: "scoring", Key: "relationship", Delta: round4(candidate.RelationshipScore * options.RelationshipWeight)},
		BehaviorReason{Source: "scoring", Key: "affect", Delta: round4(candidate.AffectScore * options.AffectWeight)},
		BehaviorReason{Source: "scoring", Key: "user_preference", Delta: round4(candidate.UserPreferenceScore * options.UserPreferenceWeight)},
		BehaviorReason{Source: "scoring", Key: "risk", Delta: round4(-candidate.RiskScore * options.RiskWeight)},
		BehaviorReason{Source: "scoring", Key: "repeat_penalty", Delta: round4(-candidate.RepeatPenalty * options.RepeatPenaltyWeight)},
		BehaviorReason{Source: "scoring", Key: "fatigue_penalty", Delta: round4(-candidate.FatiguePenalty * options.FatiguePenaltyWeight)},
	)
	return next
}

func stripSource(reasons []BehaviorReason, source string) []BehaviorReason {
	if len(reasons) == 0 {
		return nil
	}
	result := make([]BehaviorReason, 0, len(reasons))
	for _, r := range reasons {
		if r.Source == source {
			continue
		}
		result = append(result, r)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ScoreBehaviorCandidates 兼容 API:
// 对已有 Component Scores 的候选直接使用 Canonical FinalScore Formula 计算 FinalScore, 并排序
// 不调用 Context Signals, 不重算 Component (保持向后兼容)
// 新生产 Agent 主链应使用 ScoreCandidates
func ScoreBehaviorCandidates(candidates []BehaviorCandidate, options BehaviorScoringOptions) []BehaviorCandidate {
	result := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := cloneCandidate(candidate)
		next.FinalScore = computeFinalScore(next, options)
		next.ScoringVersion = BehaviorFormulaVersionV2
		result = append(result, next)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].FinalScore > result[j].FinalScore
	})
	return result
}

func SelectBehaviorCandidate(candidates []BehaviorCandidate, options BehaviorScoringOptions) BehaviorSelectionResult {
	allowed := make([]BehaviorCandidate, 0, len(candidates))
	blocked := make([]BehaviorCandidate, 0)
	diagnostics := []string{string(BehaviorFormulaVersionV1)}
	for _, candidate := range candidates {
		if blockedByHardConstraint(candidate) {
			blocked = append(blocked, candidate)
			diagnostics = append(diagnostics, "blocked:"+candidate.ID)
			continue
		}
		allowed = append(allowed, candidate)
	}
	scored := ScoreBehaviorCandidates(allowed, options)
	result := BehaviorSelectionResult{
		Blocked: blocked,
		Audit: BehaviorAudit{
			FormulaVersion: string(BehaviorFormulaVersionV1),
			Diagnostics:    diagnostics,
		},
	}
	if len(scored) == 0 {
		result.Audit.Diagnostics = append(result.Audit.Diagnostics, "no_candidate_selected")
		return result
	}
	result.Selected = scored[0]
	if len(scored) > 1 {
		result.Alternatives = scored[1:]
	}
	result.Audit.Diagnostics = append(result.Audit.Diagnostics, "selected:"+result.Selected.ID)
	return result
}

func ApplyLifeInterruptionRisk(candidates []BehaviorCandidate, life LifeSnapshot) []BehaviorCandidate {
	adjusted := make([]BehaviorCandidate, 0, len(candidates))
	busy := clamp01(life.Busy)
	if busy <= 0 {
		for _, candidate := range candidates {
			adjusted = append(adjusted, candidate)
		}
		return adjusted
	}
	for _, candidate := range candidates {
		next := candidate
		if candidate.Channel == BehaviorChannelProactive || candidate.Tag == BehaviorTagProactiveCheck {
			riskDelta := round4(busy * 0.45)
			next.RiskScore = round4(next.RiskScore + riskDelta)
			next.Reasons = append(next.Reasons, BehaviorReason{Source: "life", Key: "busy_interruption_risk", Delta: -riskDelta})
			next.Constraints = appendDedupConstraint(next.Constraints, BehaviorConstraint{Kind: "busy_interruption", Limit: 0.82, Observed: busy, Hard: busy >= 0.9})
		}
		adjusted = append(adjusted, next)
	}
	return adjusted
}

func (r AffectRegulator) ComputeMultiplier(affect AffectSignalInput) float64 {
	positive := clamp01(affect.Positive)
	negative := clamp01(affect.Negative)
	stress := clamp01(affect.Stress)
	boost := positive * r.PositiveEmotionWeight
	penalty := negative*r.NegativeEmotionWeight + stress*r.StressWeight
	raw := 1.0 + boost - penalty
	if raw > r.MaxMultiplier {
		return r.MaxMultiplier
	}
	if raw < r.MinMultiplier {
		return r.MinMultiplier
	}
	return round4(raw)
}

func blockedByHardConstraint(candidate BehaviorCandidate) bool {
	for _, constraint := range candidate.Constraints {
		if constraint.Hard && constraint.Limit > 0 && constraint.Observed > constraint.Limit {
			return true
		}
		if constraint.Hard && constraint.Kind == "blocked" {
			return true
		}
	}
	return false
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

type FusedAffectNeedSignal struct {
	Multiplier float64 `json:"multiplier"`
	AffectRaw  float64 `json:"affectRaw"`
	NeedRaw    float64 `json:"needRaw"`
	FormulaID  string  `json:"formulaId"`
}

type NeedScoringInput struct {
	Kind      string  `json:"kind"`
	Deviation float64 `json:"deviation"`
}

func ComputeAffectNeedFusion(affect AffectSignalInput, needs []NeedScoringInput) FusedAffectNeedSignal {
	pos := clamp01(affect.Positive)
	neg := clamp01(affect.Negative)
	stress := clamp01(affect.Stress)
	affectRaw := 1.0 + (pos*0.35 - neg*0.40 - stress*0.25)
	if affectRaw < 0.65 {
		affectRaw = 0.65
	}
	if affectRaw > 1.35 {
		affectRaw = 1.35
	}
	needRaw := 1.0
	if len(needs) > 0 {
		sumDev := 0.0
		for _, n := range needs {
			d := n.Deviation
			if d < 0 {
				d = 0
			}
			if d > 1 {
				d = 1
			}
			sumDev += d
		}
		avgDev := sumDev / float64(len(needs))
		needRaw = 1.0 + avgDev*0.50
		if needRaw > 1.35 {
			needRaw = 1.35
		}
	}
	multiplier := (affectRaw + needRaw) / 2.0
	if multiplier < 0.65 {
		multiplier = 0.65
	}
	if multiplier > 1.35 {
		multiplier = 1.35
	}
	return FusedAffectNeedSignal{
		Multiplier: round4(multiplier),
		AffectRaw:  round4(affectRaw),
		NeedRaw:    round4(needRaw),
		FormulaID:  "affect-need-fusion-v1",
	}
}
