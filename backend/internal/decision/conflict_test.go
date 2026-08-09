package decision

import (
	"testing"
)

func TestResolveConflictsRemovesLowerPriority(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "set_boundary", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "offer_support", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
	}
	matrix := DefaultConflictMatrix()
	resolution, err := resolveConflicts(candidates, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Candidates) != 1 {
		t.Fatalf("应剩余 1 个候选, got %d", len(resolution.Candidates))
	}
	if resolution.Candidates[0].ID != "set_boundary" {
		t.Fatalf("set_boundary 优先级更高应胜出, got %s", resolution.Candidates[0].ID)
	}
	if len(resolution.Rejected) != 1 || resolution.Rejected[0].Candidate.ID != "offer_support" {
		t.Fatalf("offer_support 应被 rejected, got %#v", resolution.Rejected)
	}
}

func TestResolveConflictsNoZeroTail(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "set_boundary", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "offer_support", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
	}
	matrix := DefaultConflictMatrix()
	resolution, err := resolveConflicts(candidates, matrix)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range resolution.Candidates {
		if c.ID == "" {
			t.Fatal("不应有零值候选")
		}
	}
}

func TestResolveConflictsEqualPriorityUsesScore(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "ask_clarify", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "tool_search", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
	}
	matrix := DefaultConflictMatrix()
	resolution, err := resolveConflicts(candidates, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Candidates) != 1 {
		t.Fatalf("应剩余 1 个候选, got %d", len(resolution.Candidates))
	}
	if resolution.Candidates[0].ID != "tool_search" {
		t.Fatalf("equal priority 时高分应胜出, got %s", resolution.Candidates[0].ID)
	}
}

func TestResolveConflictsEqualPriorityEqualScoreUsesID(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "tool_search", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "ask_clarify", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
	}
	matrix := DefaultConflictMatrix()
	resolution, err := resolveConflicts(candidates, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Candidates) != 1 {
		t.Fatalf("应剩余 1 个候选, got %d", len(resolution.Candidates))
	}
	if resolution.Candidates[0].ID != "ask_clarify" {
		t.Fatalf("equal priority+equal score 时 ID 较小者胜, got %s", resolution.Candidates[0].ID)
	}
}

func TestResolveConflictsOrderIndependent(t *testing.T) {
	matrix := DefaultConflictMatrix()

	input1 := []BehaviorCandidate{
		{ID: "set_boundary", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "offer_support", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
	}
	input2 := []BehaviorCandidate{
		{ID: "offer_support", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "set_boundary", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
	}

	res1, err := resolveConflicts(input1, matrix)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := resolveConflicts(input2, matrix)
	if err != nil {
		t.Fatal(err)
	}

	winner1 := res1.Candidates[0].ID
	winner2 := res2.Candidates[0].ID
	if winner1 != winner2 {
		t.Fatalf("输入顺序不影响结果: forward=%s reverse=%s", winner1, winner2)
	}
	if winner1 != "set_boundary" {
		t.Fatalf("set_boundary 优先级更高应胜出, got %s", winner1)
	}
}

func TestResolveConflictsThresholdBeforeConflict(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "set_boundary", FinalScore: 0.01, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "offer_support", FinalScore: 0.80, ScoringVersion: BehaviorFormulaVersionV2},
	}
	threshold := 0.10

	aboveThreshold := make([]BehaviorCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.FinalScore >= threshold {
			aboveThreshold = append(aboveThreshold, c)
		}
	}

	matrix := DefaultConflictMatrix()
	resolution, err := resolveConflicts(aboveThreshold, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Candidates) != 1 || resolution.Candidates[0].ID != "offer_support" {
		t.Fatalf("threshold 先于 conflict 过滤, 应选 offer_support, got %s", resolution.Candidates[0].ID)
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
	if rule.PriorityA != 2 {
		t.Fatalf("PriorityA for proactive_greet 应为 2, got %d", rule.PriorityA)
	}
	ruleRev := findConflictRule("wait_observe", "proactive_greet", matrix)
	if ruleRev == nil {
		t.Fatal("反向查找也应找到规则")
	}
	if ruleRev.PriorityA != 2 {
		t.Fatalf("反向查找 PriorityA 仍为 CandidateA=proactive_greet 的优先级 (2), got %d", ruleRev.PriorityA)
	}
}

func TestValidateConflictMatrixRejectsEmptyCandidateA(t *testing.T) {
	matrix := ConflictMatrix{
		Rules: []ConflictRule{{CandidateA: "", CandidateB: "x", Relation: ConflictMutualExclusive}},
	}
	if err := ValidateConflictMatrix(matrix); err == nil {
		t.Fatal("应拒绝空 CandidateA")
	}
}

func TestValidateConflictMatrixRejectsSelfOverride(t *testing.T) {
	matrix := ConflictMatrix{
		Rules: []ConflictRule{{CandidateA: "x", CandidateB: "x", Relation: ConflictMutualExclusive}},
	}
	if err := ValidateConflictMatrix(matrix); err == nil {
		t.Fatal("应拒绝 A=B 规则")
	}
}

func TestValidateConflictMatrixRejectsInvalidRelation(t *testing.T) {
	matrix := ConflictMatrix{
		Rules: []ConflictRule{{CandidateA: "a", CandidateB: "b", Relation: ConflictRelation("unknown")}},
	}
	if err := ValidateConflictMatrix(matrix); err == nil {
		t.Fatal("应拒绝无效关系")
	}
}

func TestValidateConflictMatrixRejectsDuplicatePair(t *testing.T) {
	matrix := ConflictMatrix{
		Rules: []ConflictRule{
			{CandidateA: "a", CandidateB: "b", Relation: ConflictMutualExclusive},
			{CandidateA: "b", CandidateB: "a", Relation: ConflictMutualExclusive},
		},
	}
	if err := ValidateConflictMatrix(matrix); err == nil {
		t.Fatal("应拒绝重复规则对")
	}
}
