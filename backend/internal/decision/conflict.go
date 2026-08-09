package decision

import (
	"fmt"
	"sort"
)

type ConflictRelation string

const (
	ConflictMutualExclusive ConflictRelation = "mutual_exclusive"
	ConflictIncompatible    ConflictRelation = "incompatible"
)

type ConflictRule struct {
	CandidateA string
	CandidateB string
	Relation   ConflictRelation
	PriorityA  int
	PriorityB  int
}

type ConflictMatrix struct {
	Rules []ConflictRule
}

type ConflictDecision struct {
	WinnerID       string
	LoserID        string
	Relation       ConflictRelation
	WinnerPriority int
	LoserPriority  int
	Reason         string
}

type ConflictResolution struct {
	Candidates []BehaviorCandidate
	Rejected   []ArbitrationRejection
	Decisions  []ConflictDecision
}

func DefaultConflictMatrix() ConflictMatrix {
	return ConflictMatrix{
		Rules: []ConflictRule{
			{CandidateA: "proactive_greet", CandidateB: "wait_observe", Relation: ConflictMutualExclusive, PriorityA: 2, PriorityB: 1},
			{CandidateA: "set_boundary", CandidateB: "offer_support", Relation: ConflictIncompatible, PriorityA: 3, PriorityB: 2},
			{CandidateA: "express_emotion", CandidateB: "wait_observe", Relation: ConflictMutualExclusive, PriorityA: 2, PriorityB: 1},
			{CandidateA: "ask_clarify", CandidateB: "tool_search", Relation: ConflictIncompatible, PriorityA: 2, PriorityB: 2},
		},
	}
}

func ValidateConflictMatrix(matrix ConflictMatrix) error {
	type pairKey struct{ a, b string }
	seen := make(map[pairKey]bool)
	for i, rule := range matrix.Rules {
		if rule.CandidateA == "" {
			return fmt.Errorf("conflict rule %d: CandidateA cannot be empty", i)
		}
		if rule.CandidateB == "" {
			return fmt.Errorf("conflict rule %d: CandidateB cannot be empty", i)
		}
		if rule.CandidateA == rule.CandidateB {
			return fmt.Errorf("conflict rule %d: CandidateA and CandidateB cannot be the same (%s)", i, rule.CandidateA)
		}
		if rule.Relation != ConflictMutualExclusive && rule.Relation != ConflictIncompatible {
			return fmt.Errorf("conflict rule %d: unknown relation: %s", i, rule.Relation)
		}
		if rule.PriorityA < 0 || rule.PriorityB < 0 {
			return fmt.Errorf("conflict rule %d: priority cannot be negative", i)
		}
		a, b := rule.CandidateA, rule.CandidateB
		if a > b {
			a, b = b, a
		}
		pk := pairKey{a: a, b: b}
		if seen[pk] {
			return fmt.Errorf("conflict rule %d: duplicate rule for pair (%s, %s)", i, a, b)
		}
		seen[pk] = true
	}
	return nil
}

func conflictBeats(cand BehaviorCandidate, survivor BehaviorCandidate, rule ConflictRule) (bool, conflictOutcome) {
	candPriority, _ := conflictPriorityFor(rule, cand.ID)
	survivorPriority, _ := conflictPriorityFor(rule, survivor.ID)

	if candPriority > survivorPriority {
		return true, conflictOutcome{
			decision: ConflictDecision{
				WinnerID:       cand.ID,
				LoserID:        survivor.ID,
				Relation:       rule.Relation,
				WinnerPriority: candPriority,
				LoserPriority:  survivorPriority,
				Reason:         fmt.Sprintf("conflict:%s>higher_priority", cand.ID),
			},
		}
	}
	if candPriority < survivorPriority {
		return false, conflictOutcome{
			decision: ConflictDecision{
				WinnerID:       survivor.ID,
				LoserID:        cand.ID,
				Relation:       rule.Relation,
				WinnerPriority: survivorPriority,
				LoserPriority:  candPriority,
				Reason:         fmt.Sprintf("conflict:%s>lower_priority", survivor.ID),
			},
		}
	}
	if cand.FinalScore > survivor.FinalScore {
		return true, conflictOutcome{
			decision: ConflictDecision{
				WinnerID:       cand.ID,
				LoserID:        survivor.ID,
				Relation:       rule.Relation,
				WinnerPriority: candPriority,
				LoserPriority:  survivorPriority,
				Reason:         fmt.Sprintf("conflict:%s>equal_priority_higher_score", cand.ID),
			},
		}
	}
	if cand.FinalScore < survivor.FinalScore {
		return false, conflictOutcome{
			decision: ConflictDecision{
				WinnerID:       survivor.ID,
				LoserID:        cand.ID,
				Relation:       rule.Relation,
				WinnerPriority: survivorPriority,
				LoserPriority:  candPriority,
				Reason:         fmt.Sprintf("conflict:%s>equal_priority_higher_score", survivor.ID),
			},
		}
	}
	if cand.ID < survivor.ID {
		return true, conflictOutcome{
			decision: ConflictDecision{
				WinnerID:       cand.ID,
				LoserID:        survivor.ID,
				Relation:       rule.Relation,
				WinnerPriority: candPriority,
				LoserPriority:  survivorPriority,
				Reason:         fmt.Sprintf("conflict:%s>equal_score_id_wins", cand.ID),
			},
		}
	}
	return false, conflictOutcome{
		decision: ConflictDecision{
			WinnerID:       survivor.ID,
			LoserID:        cand.ID,
			Relation:       rule.Relation,
			WinnerPriority: survivorPriority,
			LoserPriority:  candPriority,
			Reason:         fmt.Sprintf("conflict:%s>equal_score_id_wins", survivor.ID),
		},
	}
}

type conflictOutcome struct {
	decision ConflictDecision
}

func resolveConflicts(candidates []BehaviorCandidate, matrix ConflictMatrix) (ConflictResolution, error) {
	resolution := ConflictResolution{
		Candidates: make([]BehaviorCandidate, 0, len(candidates)),
		Rejected:   make([]ArbitrationRejection, 0),
		Decisions:  make([]ConflictDecision, 0),
	}
	if len(candidates) < 2 {
		resolution.Candidates = append(resolution.Candidates, candidates...)
		return resolution, nil
	}

	sorted := make([]BehaviorCandidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].FinalScore != sorted[j].FinalScore {
			return sorted[i].FinalScore > sorted[j].FinalScore
		}
		return sorted[i].ID < sorted[j].ID
	})

	survivors := make([]BehaviorCandidate, 0, len(sorted))
	for _, cand := range sorted {
		lost := false
		var lostOutcome conflictOutcome
		for _, survivor := range survivors {
			rule := findConflictRule(cand.ID, survivor.ID, matrix)
			if rule == nil {
				continue
			}
			beats, outcome := conflictBeats(cand, survivor, *rule)
			if !beats {
				lost = true
				lostOutcome = outcome
				break
			}
		}

		if lost {
			resolution.Rejected = append(resolution.Rejected, ArbitrationRejection{
				Candidate: cand,
				Stage:     ArbitrationRejectConflict,
				Reason:    lostOutcome.decision.Reason,
			})
			resolution.Decisions = append(resolution.Decisions, lostOutcome.decision)
			continue
		}

		newSurvivors := make([]BehaviorCandidate, 0, len(survivors)+1)
		for _, survivor := range survivors {
			rule := findConflictRule(cand.ID, survivor.ID, matrix)
			if rule == nil {
				newSurvivors = append(newSurvivors, survivor)
				continue
			}
			beats, outcome := conflictBeats(cand, survivor, *rule)
			if beats {
				resolution.Rejected = append(resolution.Rejected, ArbitrationRejection{
					Candidate: survivor,
					Stage:     ArbitrationRejectConflict,
					Reason:    outcome.decision.Reason,
				})
				resolution.Decisions = append(resolution.Decisions, outcome.decision)
			} else {
				newSurvivors = append(newSurvivors, survivor)
			}
		}
		newSurvivors = append(newSurvivors, cand)
		survivors = newSurvivors
	}

	resolution.Candidates = survivors
	return resolution, nil
}

func conflictPriorityFor(rule ConflictRule, candidateID string) (int, bool) {
	if candidateID == rule.CandidateA {
		return rule.PriorityA, true
	}
	if candidateID == rule.CandidateB {
		return rule.PriorityB, true
	}
	return 0, false
}

func findConflictRule(idA, idB string, matrix ConflictMatrix) *ConflictRule {
	for i := range matrix.Rules {
		rule := &matrix.Rules[i]
		if (rule.CandidateA == idA && rule.CandidateB == idB) ||
			(rule.CandidateA == idB && rule.CandidateB == idA) {
			return rule
		}
	}
	return nil
}

func CanCoexist(candidateA, candidateB BehaviorCandidate, matrix ConflictMatrix) bool {
	rule := findConflictRule(candidateA.ID, candidateB.ID, matrix)
	return rule == nil
}
