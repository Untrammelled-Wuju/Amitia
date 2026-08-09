package decision

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

type ArbitrationDisposition string

const (
	ArbitrationDispositionSelected    ArbitrationDisposition = "selected"
	ArbitrationDispositionFallback    ArbitrationDisposition = "fallback"
	ArbitrationDispositionNoSelection ArbitrationDisposition = "no_selection"
)

type ArbitrationRejectionStage string

const (
	ArbitrationRejectThreshold ArbitrationRejectionStage = "threshold"
	ArbitrationRejectOverride  ArbitrationRejectionStage = "override"
	ArbitrationRejectConflict  ArbitrationRejectionStage = "conflict"
)

type ArbitrationRejection struct {
	Candidate BehaviorCandidate         `json:"candidate"`
	Stage     ArbitrationRejectionStage `json:"stage"`
	Reason    string                    `json:"reason"`
}

type ArbitrationConfig struct {
	MinScoreThreshold      float64
	FallbackCandidateID    string
	RequiredScoringVersion string
}

func DefaultArbitrationConfig() ArbitrationConfig {
	return ArbitrationConfig{
		MinScoreThreshold:      0.10,
		FallbackCandidateID:    "wait_observe",
		RequiredScoringVersion: string(BehaviorFormulaVersionV2),
	}
}

func ValidateArbitrationConfig(config ArbitrationConfig) error {
	if math.IsNaN(config.MinScoreThreshold) || math.IsInf(config.MinScoreThreshold, 0) {
		return errors.New("arbitration: MinScoreThreshold must be finite")
	}
	if config.FallbackCandidateID == "" {
		return errors.New("arbitration: FallbackCandidateID cannot be empty")
	}
	if config.RequiredScoringVersion == "" {
		return errors.New("arbitration: RequiredScoringVersion cannot be empty")
	}
	return nil
}

type ArbitrationInput struct {
	Candidates []BehaviorCandidate
	Filter     HardConstraintFilter
	Now        time.Time

	Goals        []Goal
	Intentions   []Intention
	Relationship RelationshipSnapshot
	Psyche       PsycheSignalSet
	Life         LifeSnapshot
	History      BehaviorHistory
}

type ArbitrationResult struct {
	Selected       BehaviorCandidate
	HasSelection   bool
	Disposition    ArbitrationDisposition
	Alternatives   []BehaviorCandidate
	Blocked        []BehaviorCandidate
	Rejected       []ArbitrationRejection
	Conflicts      []ConflictDecision
	ConflictLog    []string
	FallbackUsed   bool
	FallbackReason string
	Audit          BehaviorAudit
}

type ArbitrationLayer struct {
	Config    ArbitrationConfig
	Conflicts ConflictMatrix
}

func NewArbitrationLayer(config ArbitrationConfig) ArbitrationLayer {
	return ArbitrationLayer{
		Config:    config,
		Conflicts: DefaultConflictMatrix(),
	}
}

func NewArbitrationLayerWithConflicts(config ArbitrationConfig, matrix ConflictMatrix) ArbitrationLayer {
	return ArbitrationLayer{
		Config:    config,
		Conflicts: matrix,
	}
}

func DefaultArbitrationLayer() ArbitrationLayer {
	return NewArbitrationLayer(DefaultArbitrationConfig())
}

func (a *ArbitrationLayer) Arbitrate(input ArbitrationInput) (ArbitrationResult, error) {
	if err := a.validateInput(input); err != nil {
		return ArbitrationResult{}, err
	}

	allowed, blocked := input.Filter.Filter(input.Candidates, input.Now, input.History)

	viable, belowThreshold := partitionByThreshold(allowed, a.Config.MinScoreThreshold)

	viable, overrideRejected, err := resolveOverrides(viable)
	if err != nil {
		return ArbitrationResult{}, err
	}

	resolution, err := resolveConflicts(viable, a.Conflicts)
	if err != nil {
		return ArbitrationResult{}, err
	}

	ranked := sortCandidatesForArbitration(resolution.Candidates)

	var allRejected []ArbitrationRejection
	allRejected = append(allRejected, belowThreshold...)
	allRejected = append(allRejected, overrideRejected...)
	allRejected = append(allRejected, resolution.Rejected...)

	conflictLog := make([]string, 0, len(resolution.Decisions))
	for _, dec := range resolution.Decisions {
		conflictLog = append(conflictLog, dec.Reason)
	}

	audit := BehaviorAudit{
		FormulaVersion: a.Config.RequiredScoringVersion,
		ConflictIDs:    conflictLog,
	}

	if len(ranked) > 0 {
		selected := ranked[0]
		audit.Diagnostics = append(audit.Diagnostics, "selected:"+selected.ID)
		var alternatives []BehaviorCandidate
		if len(ranked) > 1 {
			alternatives = ranked[1:]
		}
		return ArbitrationResult{
			Selected:     selected,
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Alternatives: alternatives,
			Blocked:      blocked,
			Rejected:     allRejected,
			Conflicts:    resolution.Decisions,
			ConflictLog:  conflictLog,
			Audit:        audit,
		}, nil
	}

	fallback, ok := selectFallbackCandidate(allowed, a.Config.FallbackCandidateID)
	if ok {
		audit.Diagnostics = append(audit.Diagnostics, "fallback:"+fallback.ID)
		finalRejected := removeFromRejected(allRejected, fallback.ID)
		return ArbitrationResult{
			Selected:       fallback,
			HasSelection:   true,
			Disposition:    ArbitrationDispositionFallback,
			Blocked:        blocked,
			Rejected:       finalRejected,
			Conflicts:      resolution.Decisions,
			ConflictLog:    conflictLog,
			FallbackUsed:   true,
			FallbackReason: "below_threshold_fallback",
			Audit:          audit,
		}, nil
	}

	audit.Diagnostics = append(audit.Diagnostics, "no_selection:all_hard_blocked")
	var allBlocked []BehaviorCandidate
	allBlocked = append(allBlocked, blocked...)
	for _, rej := range belowThreshold {
		allBlocked = append(allBlocked, rej.Candidate)
	}
	return ArbitrationResult{
		HasSelection: false,
		Disposition:  ArbitrationDispositionNoSelection,
		Blocked:      allBlocked,
		Rejected:     allRejected,
		Conflicts:    resolution.Decisions,
		ConflictLog:  conflictLog,
		Audit:        audit,
	}, nil
}

func (a *ArbitrationLayer) validateInput(input ArbitrationInput) error {
	if err := ValidateArbitrationConfig(a.Config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if input.Now.IsZero() {
		return errors.New("arbitration: Now is required")
	}
	seen := make(map[string]bool)
	for _, candidate := range input.Candidates {
		if candidate.ID == "" {
			return errors.New("arbitration: candidate ID cannot be empty")
		}
		if seen[candidate.ID] {
			return fmt.Errorf("arbitration: duplicate candidate id: %s", candidate.ID)
		}
		seen[candidate.ID] = true
		if math.IsNaN(candidate.FinalScore) || math.IsInf(candidate.FinalScore, 0) {
			return fmt.Errorf("arbitration: candidate %s has non-finite FinalScore", candidate.ID)
		}
		if a.Config.RequiredScoringVersion != "" && candidate.ScoringVersion != a.Config.RequiredScoringVersion {
			return fmt.Errorf("arbitration: candidate %s ScoringVersion=%s, required %s",
				candidate.ID, candidate.ScoringVersion, a.Config.RequiredScoringVersion)
		}
	}
	return nil
}

func partitionByThreshold(candidates []BehaviorCandidate, threshold float64) ([]BehaviorCandidate, []ArbitrationRejection) {
	allowed := make([]BehaviorCandidate, 0, len(candidates))
	rejected := make([]ArbitrationRejection, 0)
	for _, cand := range candidates {
		if cand.FinalScore >= threshold {
			allowed = append(allowed, cand)
		} else {
			rejected = append(rejected, ArbitrationRejection{
				Candidate: cand,
				Stage:     ArbitrationRejectThreshold,
				Reason:    fmt.Sprintf("below_threshold: %f < %f", cand.FinalScore, threshold),
			})
		}
	}
	return allowed, rejected
}

func resolveOverrides(candidates []BehaviorCandidate) ([]BehaviorCandidate, []ArbitrationRejection, error) {
	candidatesByID := make(map[string]BehaviorCandidate)
	for _, cand := range candidates {
		candidatesByID[cand.ID] = cand
	}

	overriddenBy := make(map[string]string)
	for _, cand := range candidates {
		for _, target := range cand.Overrides {
			if target == cand.ID {
				return nil, nil, fmt.Errorf("arbitration: candidate %s overrides itself", cand.ID)
			}
			if _, exists := candidatesByID[target]; !exists {
				continue
			}
			if existingOverrider, exists := overriddenBy[target]; exists && existingOverrider != cand.ID {
				return nil, nil, fmt.Errorf("arbitration: multiple candidates override target %s", target)
			}
			overriddenBy[target] = cand.ID
		}
	}

	for target, overrider := range overriddenBy {
		if _, exists := overriddenBy[overrider]; exists && target != overrider {
			return nil, nil, fmt.Errorf("arbitration: mutual override between %s and %s", overrider, target)
		}
	}

	survivors := make([]BehaviorCandidate, 0, len(candidates))
	rejected := make([]ArbitrationRejection, 0)
	for _, cand := range candidates {
		if overrider, isOverridden := overriddenBy[cand.ID]; isOverridden {
			rejected = append(rejected, ArbitrationRejection{
				Candidate: cand,
				Stage:     ArbitrationRejectOverride,
				Reason:    fmt.Sprintf("override:loser_of_%s", overrider),
			})
		} else {
			survivors = append(survivors, cand)
		}
	}

	return survivors, rejected, nil
}

func selectFallbackCandidate(allowed []BehaviorCandidate, fallbackID string) (BehaviorCandidate, bool) {
	for _, cand := range allowed {
		if cand.ID == fallbackID {
			return cand, true
		}
	}
	return BehaviorCandidate{}, false
}

func removeFromRejected(rejected []ArbitrationRejection, candidateID string) []ArbitrationRejection {
	result := make([]ArbitrationRejection, 0, len(rejected))
	for _, rej := range rejected {
		if rej.Candidate.ID == candidateID {
			continue
		}
		result = append(result, rej)
	}
	return result
}

func compareArbitrationCandidates(a, b BehaviorCandidate) int {
	if a.FinalScore != b.FinalScore {
		if a.FinalScore > b.FinalScore {
			return -1
		}
		return 1
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
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

func sortCandidatesForArbitration(candidates []BehaviorCandidate) []BehaviorCandidate {
	result := make([]BehaviorCandidate, len(candidates))
	copy(result, candidates)
	sort.SliceStable(result, func(i, j int) bool {
		return compareArbitrationCandidates(result[i], result[j]) < 0
	})
	return result
}
