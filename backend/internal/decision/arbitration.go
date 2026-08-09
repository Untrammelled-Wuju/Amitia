package decision

import "sort"

import "time"

type ArbitrationConfig struct {
	MinScoreThreshold   float64
	UseConflictCheck    bool
	UseConsistencyCheck bool
}

func DefaultArbitrationConfig() ArbitrationConfig {
	return ArbitrationConfig{
		MinScoreThreshold:   0.1,
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
	Now          time.Time
	Filter       HardConstraintFilter
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

// Arbitrate B4 只保留: HardConstraint/Conflict/Sort/Selection
// Candidates 必须已经由 ScoreCandidates 评分完成
func (a *ArbitrationLayer) Arbitrate(input ArbitrationInput) ArbitrationResult {
	allowed, blocked := input.Filter.Filter(input.Candidates, input.Now)
	audit := BehaviorAudit{
		FormulaVersion: BehaviorFormulaVersionV2,
	}

	if len(allowed) == 0 {
		fallback := buildFallbackCandidate()
		return ArbitrationResult{
			Selected:     fallback,
			Blocked:      blocked,
			FallbackUsed: true,
			Audit:        audit,
		}
	}

	sorted := a.sortByScore(allowed)

	if a.Config.UseConflictCheck {
		conflictMatrix := DefaultConflictMatrix()
		conflictLog := ResolveConflicts(sorted, conflictMatrix)
		audit.ConflictIDs = conflictLog
	}

	if len(sorted) > 0 && sorted[0].FinalScore < a.Config.MinScoreThreshold {
		fallback := buildFallbackCandidate()
		audit.Diagnostics = append(audit.Diagnostics, "below_threshold_fallback")
		return ArbitrationResult{
			Selected:     fallback,
			Alternatives: sorted,
			Blocked:      blocked,
			FallbackUsed: true,
			Audit:        audit,
		}
	}

	selected := sorted[0]
	var alternatives []BehaviorCandidate
	if len(sorted) > 1 {
		alternatives = sorted[1:]
	}
	return ArbitrationResult{
		Selected:     selected,
		Alternatives: alternatives,
		Blocked:      blocked,
		Audit:        audit,
	}
}

func (a *ArbitrationLayer) sortByScore(candidates []BehaviorCandidate) []BehaviorCandidate {
	result := make([]BehaviorCandidate, len(candidates))
	copy(result, candidates)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].FinalScore > result[j].FinalScore
	})
	return result
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

// candidateHasCanonicalScore 检查候选是否经过 B4 评分
func candidateHasCanonicalScore(candidate BehaviorCandidate) bool {
	return candidate.ScoringVersion == BehaviorFormulaVersionV2
}
