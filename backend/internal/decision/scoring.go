package decision

import (
	"math"
	"sort"
)

const (
	BehaviorFormulaVersionV1 = "behavior-scoring-v1"
)

type BehaviorScoringOptions struct {
	BaseWeight         float64
	PersonalityWeight  float64
	NeedWeight         float64
	RelationshipWeight float64
	AffectWeight       float64
	RiskWeight         float64
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
		BaseWeight:         1,
		PersonalityWeight:  1,
		NeedWeight:         1,
		RelationshipWeight: 1,
		AffectWeight:       1,
		RiskWeight:         1,
	}
}

func ScoreBehaviorCandidates(candidates []BehaviorCandidate, options BehaviorScoringOptions) []BehaviorCandidate {
	options = normalizeScoringOptions(options)
	scored := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		next.FinalScore = scoreBehaviorCandidate(next, options)
		next.Reasons = appendScoreReasons(next.Reasons, next, options)
		scored = append(scored, next)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].FinalScore > scored[j].FinalScore
	})
	return scored
}

func SelectBehaviorCandidate(candidates []BehaviorCandidate, options BehaviorScoringOptions) BehaviorSelectionResult {
	options = normalizeScoringOptions(options)
	allowed := make([]BehaviorCandidate, 0, len(candidates))
	blocked := make([]BehaviorCandidate, 0)
	diagnostics := []string{string(BehaviorFormulaVersionV1)}
	for _, candidate := range candidates {
		if blockedByHardConstraint(candidate) {
			next := candidate
			next.FinalScore = scoreBehaviorCandidate(next, options)
			blocked = append(blocked, next)
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
			next.Constraints = append(next.Constraints, BehaviorConstraint{Kind: "busy_interruption", Limit: 0.82, Observed: busy, Hard: busy >= 0.9})
		}
		adjusted = append(adjusted, next)
	}
	return adjusted
}

func ScoreWithAffectRegulation(candidates []BehaviorCandidate, options BehaviorScoringOptions, regulator AffectRegulator, affect AffectSignalInput) []BehaviorCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	regulator = normalizeAffectRegulator(regulator)
	multiplier := regulator.ComputeMultiplier(affect)
	affectScore := round4(multiplier - 1.0)
	scored := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		next.AffectScore = affectScore
		next.FinalScore = scoreBehaviorCandidate(next, options)
		next.Reasons = appendScoreReasons(next.Reasons, next, options)
		scored = append(scored, next)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].FinalScore > scored[j].FinalScore
	})
	return scored
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

func scoreBehaviorCandidate(candidate BehaviorCandidate, options BehaviorScoringOptions) float64 {
	value := candidate.BaseScore*options.BaseWeight +
		candidate.PersonalityScore*options.PersonalityWeight +
		candidate.NeedScore*options.NeedWeight +
		candidate.RelationshipScore*options.RelationshipWeight +
		candidate.AffectScore*options.AffectWeight -
		candidate.RiskScore*options.RiskWeight
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
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

func normalizeScoringOptions(options BehaviorScoringOptions) BehaviorScoringOptions {
	defaults := DefaultBehaviorScoringOptions()
	if options.BaseWeight == 0 {
		options.BaseWeight = defaults.BaseWeight
	}
	if options.PersonalityWeight == 0 {
		options.PersonalityWeight = defaults.PersonalityWeight
	}
	if options.NeedWeight == 0 {
		options.NeedWeight = defaults.NeedWeight
	}
	if options.RelationshipWeight == 0 {
		options.RelationshipWeight = defaults.RelationshipWeight
	}
	if options.AffectWeight == 0 {
		options.AffectWeight = defaults.AffectWeight
	}
	if options.RiskWeight == 0 {
		options.RiskWeight = defaults.RiskWeight
	}
	return options
}

func normalizeAffectRegulator(r AffectRegulator) AffectRegulator {
	defaults := DefaultAffectRegulator()
	if r.PositiveEmotionWeight <= 0 {
		r.PositiveEmotionWeight = defaults.PositiveEmotionWeight
	}
	if r.NegativeEmotionWeight <= 0 {
		r.NegativeEmotionWeight = defaults.NegativeEmotionWeight
	}
	if r.StressWeight <= 0 {
		r.StressWeight = defaults.StressWeight
	}
	if r.MaxMultiplier <= 0 {
		r.MaxMultiplier = defaults.MaxMultiplier
	}
	if r.MinMultiplier <= 0 {
		r.MinMultiplier = defaults.MinMultiplier
	}
	return r
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

func appendScoreReasons(reasons []BehaviorReason, candidate BehaviorCandidate, options BehaviorScoringOptions) []BehaviorReason {
	next := append([]BehaviorReason{}, reasons...)
	next = append(next,
		BehaviorReason{Source: "decision", Key: "base", Delta: candidate.BaseScore * options.BaseWeight},
		BehaviorReason{Source: "decision", Key: "personality", Delta: candidate.PersonalityScore * options.PersonalityWeight},
		BehaviorReason{Source: "decision", Key: "need", Delta: candidate.NeedScore * options.NeedWeight},
		BehaviorReason{Source: "decision", Key: "relationship", Delta: candidate.RelationshipScore * options.RelationshipWeight},
		BehaviorReason{Source: "decision", Key: "affect", Delta: candidate.AffectScore * options.AffectWeight},
		BehaviorReason{Source: "decision", Key: "risk", Delta: -candidate.RiskScore * options.RiskWeight},
	)
	return next
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

func ScoreWithAffectNeedFusion(candidates []BehaviorCandidate, options BehaviorScoringOptions, affect AffectSignalInput, needInputs []NeedScoringInput) []BehaviorCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	options = normalizeScoringOptions(options)
	fusion := ComputeAffectNeedFusion(affect, needInputs)
	scored := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		rawScore := scoreBehaviorCandidate(next, options)
		next.FinalScore = round4(rawScore * fusion.Multiplier)
		next.Reasons = appendScoreReasons(next.Reasons, next, options)
		next.Reasons = append(next.Reasons, BehaviorReason{
			Source: "decision",
			Key:    "affect_need_fusion",
			Delta:  fusion.Multiplier,
		})
		scored = append(scored, next)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].FinalScore > scored[j].FinalScore
	})
	return scored
}
