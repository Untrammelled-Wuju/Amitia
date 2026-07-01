package decision

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

func ResolveConflicts(candidates []BehaviorCandidate, matrix ConflictMatrix) []string {
	conflictLog := make([]string, 0)
	if len(candidates) < 2 {
		return conflictLog
	}
	removed := make(map[int]bool)
	for i := 0; i < len(candidates); i++ {
		if removed[i] {
			continue
		}
		for j := i + 1; j < len(candidates); j++ {
			if removed[j] {
				continue
			}
			rule := findConflictRule(candidates[i].ID, candidates[j].ID, matrix)
			if rule == nil {
				continue
			}
			if rule.PriorityA >= rule.PriorityB {
				removed[j] = true
				conflictLog = append(conflictLog, "resolved:"+candidates[i].ID+">"+candidates[j].ID)
			} else {
				removed[i] = true
				conflictLog = append(conflictLog, "resolved:"+candidates[j].ID+">"+candidates[i].ID)
				break
			}
		}
	}
	if len(removed) > 0 {
		cleanCandidates := make([]BehaviorCandidate, 0, len(candidates)-len(removed))
		for idx, c := range candidates {
			if !removed[idx] {
				cleanCandidates = append(cleanCandidates, c)
			}
		}
		copy(candidates, cleanCandidates)
		for i := len(cleanCandidates); i < len(candidates); i++ {
			candidates[i] = BehaviorCandidate{}
		}
		_ = candidates[:len(cleanCandidates)]
	}
	return conflictLog
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
