package decision

import "testing"

func TestEvaluateCandidateUtilitiesReturnsAllCandidates(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", BaseScore: 0.6},
		{ID: "proactive_greet", BaseScore: 0.3},
		{ID: "set_boundary", BaseScore: 0.2},
	}
	ctx := UtilityScoringContext{}
	weights := DefaultUtilityWeightConfig()
	results := EvaluateCandidateUtilities(candidates, ctx, weights)
	if len(results) != 3 {
		t.Fatalf("结果数量应与候选一致: expected 3, got %d", len(results))
	}
}

func TestEvaluateCandidateUtilitiesPreservesInputOrder(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "wait_observe", BaseScore: 0.1},
		{ID: "chat_reply", BaseScore: 0.6},
	}
	ctx := UtilityScoringContext{
		Goals: []Goal{
			{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive, Priority: GoalPriorityHigh},
		},
	}
	weights := DefaultUtilityWeightConfig()
	results := EvaluateCandidateUtilities(candidates, ctx, weights)
	// B4 spec: EvaluateCandidateUtilities should NOT sort, preserves input order
	if results[0].CandidateID != "wait_observe" {
		t.Fatalf("EvaluateCandidateUtilities should preserve input order, got %s first", results[0].CandidateID)
	}
}

func TestEvaluateCandidateUtilitiesAllDimensionsPresent(t *testing.T) {
	candidates := []BehaviorCandidate{{ID: "chat_reply", BaseScore: 0.5}}
	ctx := UtilityScoringContext{
		Goals: []Goal{
			{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive, Priority: GoalPriorityHigh},
		},
		Intentions: []Intention{
			{GoalID: "g1", GoalType: GoalTypeConnection, Commitment: CommitmentStrong, Status: IntentionStatusExecuting},
		},
	}
	weights := DefaultUtilityWeightConfig()
	results := EvaluateCandidateUtilities(candidates, ctx, weights)
	if len(results) != 1 {
		t.Fatal("应有 1 个结果")
	}
	result := results[0]
	objectiveNames := make(map[UtilityObjective]bool)
	for _, s := range result.Scores {
		objectiveNames[s.Objective] = true
	}
	expectedObjectives := []UtilityObjective{
		UtilityGoalAlignment,
		UtilityIntentionCommitment,
		UtilityRelationshipHarmony,
		UtilityEmotionalBalance,
		UtilitySafetyCompliance,
		UtilityLifeFit,
	}
	for _, obj := range expectedObjectives {
		if !objectiveNames[obj] {
			t.Fatalf("缺少目标维度: %s", obj)
		}
	}
	if result.Composite <= 0 {
		t.Fatal("综合分应 > 0")
	}
}

func TestEvaluateCandidateUtilitiesDoesNotModifyFinalScore(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", BaseScore: 0.6, FinalScore: 0.5},
	}
	ctx := UtilityScoringContext{}
	weights := DefaultUtilityWeightConfig()
	EvaluateCandidateUtilities(candidates, ctx, weights)
	if candidates[0].FinalScore != 0.5 {
		t.Fatalf("EvaluateCandidateUtilities should not modify FinalScore, got %f", candidates[0].FinalScore)
	}
}

func TestEmptyGoalsStillReturnsDiagnostics(t *testing.T) {
	candidates := []BehaviorCandidate{{ID: "chat_reply", BaseScore: 0.6}}
	ctx := UtilityScoringContext{}
	weights := DefaultUtilityWeightConfig()
	results := EvaluateCandidateUtilities(candidates, ctx, weights)
	if len(results) != 1 {
		t.Fatal("应有 1 个结果")
	}
}
