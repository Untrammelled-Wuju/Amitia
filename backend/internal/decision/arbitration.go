package decision

import "time"

type ArbitrationConfig struct {
	MinScoreThreshold   float64
	UseSoftPreferences  bool
	UseConflictCheck    bool
	UseConsistencyCheck bool
}

func DefaultArbitrationConfig() ArbitrationConfig {
	return ArbitrationConfig{
		MinScoreThreshold:   0.1,
		UseSoftPreferences:  true,
		UseConflictCheck:    true,
		UseConsistencyCheck: true,
	}
}

type ArbitrationInput struct {
	Candidates   []BehaviorCandidate
	Goals        []Goal
	Intentions   []Intention
	Relationship RelationshipSnapshot
	Psyche       PsycheSignalSet
	Life         LifeSnapshot
	History      BehaviorHistory
	SoftPrefs    SoftPreferenceInput
	Filter       HardConstraintFilter
	Now          time.Time
}

type ArbitrationResult struct {
	Selected     BehaviorCandidate
	Alternatives []BehaviorCandidate
	Blocked      []BehaviorCandidate
	ConflictLog  []string
	FallbackUsed bool
	Audit        BehaviorAudit
}

type ArbitrationLayer struct {
	Config ArbitrationConfig
}

func NewArbitrationLayer(config ArbitrationConfig) ArbitrationLayer {
	return ArbitrationLayer{Config: config}
}

func DefaultArbitrationLayer() ArbitrationLayer {
	return NewArbitrationLayer(DefaultArbitrationConfig())
}

func (a *ArbitrationLayer) Arbitrate(input ArbitrationInput) ArbitrationResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	allowed, blocked := input.Filter.Filter(input.Candidates, now)
	audit := BehaviorAudit{
		FormulaVersion: string(BehaviorFormulaVersionV1),
	}
	if len(allowed) == 0 {
		fallback := buildFallbackCandidate()
		return ArbitrationResult{
			Selected:     fallback,
			Alternatives: nil,
			Blocked:      blocked,
			FallbackUsed: true,
			Audit:        audit,
		}
	}
	scored := ScoreBehaviorCandidates(allowed, DefaultBehaviorScoringOptions())
	if a.Config.UseSoftPreferences {
		scored = ApplySoftPreferencesToAll(scored, input.SoftPrefs, DefaultSoftPreferenceConfig())
		sortCandidatesByScore(scored)
	}
	scored = ApplyBehaviorCostPenalties(scored, input.History, now)
	sortCandidatesByScore(scored)
	if a.Config.UseConflictCheck {
		conflictMatrix := DefaultConflictMatrix()
		conflictLog := ResolveConflicts(scored, conflictMatrix)
		audit.ConflictIDs = conflictLog
	}
	if len(scored) > 0 && scored[0].FinalScore < a.Config.MinScoreThreshold {
		fallback := buildFallbackCandidate()
		audit.Diagnostics = append(audit.Diagnostics, "below_threshold_fallback")
		return ArbitrationResult{
			Selected:     fallback,
			Alternatives: scored,
			Blocked:      blocked,
			FallbackUsed: true,
			Audit:        audit,
		}
	}
	selected := scored[0]
	var alternatives []BehaviorCandidate
	if len(scored) > 1 {
		alternatives = scored[1:]
	}
	return ArbitrationResult{
		Selected:     selected,
		Alternatives: alternatives,
		Blocked:      blocked,
		FallbackUsed: false,
		Audit:        audit,
	}
}

func ApplyBehaviorCostPenalties(candidates []BehaviorCandidate, history BehaviorHistory, now time.Time) []BehaviorCandidate {
	repeatConfig := DefaultRepeatPenaltyConfig()
	fatigueConfig := DefaultFatiguePenaltyConfig()
	result := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		repeatPenalty := ComputeRepeatPenalty(history, candidate.ID, now, repeatConfig)
		fatiguePenalty := ComputeFatiguePenalty(history, candidate.ID, now, fatigueConfig)
		totalPenalty := repeatPenalty + fatiguePenalty
		next.FinalScore = round4(next.FinalScore - totalPenalty)
		if next.FinalScore < 0 {
			next.FinalScore = 0
		}
		result = append(result, next)
	}
	return result
}

func sortCandidatesByScore(candidates []BehaviorCandidate) {
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].FinalScore > candidates[i].FinalScore {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
}

func buildFallbackCandidate() BehaviorCandidate {
	return BehaviorCandidate{
		ID:         "wait_observe",
		Tag:        BehaviorTagDelay,
		Channel:    BehaviorChannelSystem,
		BaseScore:  0.10,
		FinalScore: 0.10,
		Reasons: []BehaviorReason{
			{Source: "arbitration", Key: "fallback", Delta: 0},
		},
	}
}
