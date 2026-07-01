package decision

import "testing"

func TestScoreWithMultiObjectiveReturnsAllCandidates(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", BaseScore: 0.6},
		{ID: "proactive_greet", BaseScore: 0.3},
		{ID: "set_boundary", BaseScore: 0.2},
	}
	ctx := UtilityScoringContext{}
	weights := DefaultUtilityWeightConfig()
	results := ScoreWithMultiObjective(candidates, ctx, weights)
	if len(results) != 3 {
		t.Fatalf("结果数量应与候选一致: expected 3, got %d", len(results))
	}
}

func TestScoreWithMultiObjectiveSortsByComposite(t *testing.T) {
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
	results := ScoreWithMultiObjective(candidates, ctx, weights)
	if results[0].CandidateID != "chat_reply" {
		t.Fatalf("有 Connection 目标时 chat_reply 应排第一: %#v", results)
	}
	if results[0].Composite <= results[1].Composite {
		t.Fatalf("排序错误: first=%f second=%f", results[0].Composite, results[1].Composite)
	}
}

func TestScoreWithMultiObjectiveAllDimensionsPresent(t *testing.T) {
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
	results := ScoreWithMultiObjective(candidates, ctx, weights)
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

func TestGoalAlignmentWithActiveGoal(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "ask_clarify", BaseScore: 0.4},
		{ID: "chat_reply", BaseScore: 0.6},
	}
	ctx := UtilityScoringContext{
		Goals: []Goal{
			{ID: "g1", Type: GoalTypeClarification, Status: GoalStatusActive, Priority: GoalPriorityCritical},
		},
	}
	weights := DefaultUtilityWeightConfig()
	results := ScoreWithMultiObjective(candidates, ctx, weights)
	if results[0].CandidateID != "ask_clarify" {
		t.Fatalf("Clarification 目标应使 ask_clarify 排第一: %#v", results)
	}
}

func TestEmptyGoalsReturnsDefault(t *testing.T) {
	candidates := []BehaviorCandidate{{ID: "chat_reply", BaseScore: 0.6}}
	ctx := UtilityScoringContext{}
	weights := DefaultUtilityWeightConfig()
	results := ScoreWithMultiObjective(candidates, ctx, weights)
	if len(results) != 1 {
		t.Fatal("应有 1 个结果")
	}
}

func TestSafetyCompliancePenalizesToolSearch(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "tool_search", BaseScore: 0.5},
		{ID: "chat_reply", BaseScore: 0.5},
	}
	ctx := UtilityScoringContext{}
	weights := DefaultUtilityWeightConfig()
	results := ScoreWithMultiObjective(candidates, ctx, weights)
	if results[0].CandidateID != "chat_reply" {
		t.Fatalf("安全合规维度应使 tool_search 排名靠后: %#v", results)
	}
}

func TestLifeFitPenalizesProactiveWhenBusy(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "proactive_greet", BaseScore: 0.5},
		{ID: "chat_reply", BaseScore: 0.5},
	}
	ctx := UtilityScoringContext{
		Life: LifeSnapshot{Busy: 0.9, Energy: 0.7},
	}
	weights := DefaultUtilityWeightConfig()
	results := ScoreWithMultiObjective(candidates, ctx, weights)
	if results[0].CandidateID != "chat_reply" {
		t.Fatalf("忙碌状态下 proactive 应排在 chat_reply 后面: %#v", results)
	}
}
