package decision

import "testing"

func TestResolveConflictsRemovesLowerPriority(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "proactive_greet", FinalScore: 0.7},
		{ID: "wait_observe", FinalScore: 0.3},
	}
	matrix := DefaultConflictMatrix()
	log := ResolveConflicts(candidates, matrix)
	if len(log) == 0 {
		t.Fatal("应产生冲突日志")
	}
}

func TestCanCoexistAllowsUnrelated(t *testing.T) {
	candidateA := BehaviorCandidate{ID: "chat_reply"}
	candidateB := BehaviorCandidate{ID: "ask_clarify"}
	matrix := DefaultConflictMatrix()
	if !CanCoexist(candidateA, candidateB, matrix) {
		t.Fatal("不冲突的候选应可共存")
	}
}

func TestCanCoexistDetectsConflict(t *testing.T) {
	candidateA := BehaviorCandidate{ID: "proactive_greet"}
	candidateB := BehaviorCandidate{ID: "wait_observe"}
	matrix := DefaultConflictMatrix()
	if CanCoexist(candidateA, candidateB, matrix) {
		t.Fatal("proactive_greet 和 wait_observe 互斥不应共存")
	}
}

func TestDefaultConflictMatrixHasRules(t *testing.T) {
	matrix := DefaultConflictMatrix()
	if len(matrix.Rules) == 0 {
		t.Fatal("默认冲突矩阵应有规则")
	}
}

func TestFindConflictRuleSymmetric(t *testing.T) {
	matrix := DefaultConflictMatrix()
	rule := findConflictRule("proactive_greet", "wait_observe", matrix)
	if rule == nil {
		t.Fatal("正向查找应找到规则")
	}
	ruleRev := findConflictRule("wait_observe", "proactive_greet", matrix)
	if ruleRev == nil {
		t.Fatal("反向查找也应找到规则")
	}
}
