package decision

import (
	"testing"
	"time"
)

func TestConsistencyVerificationPassesForAlignedPlan(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:     PlanVersionV1,
		ID:          "plan-1",
		CreatedAt:   time.Now().UTC(),
		Selected:    BehaviorCandidate{ID: "chat_reply", RiskScore: 0.1},
		SafetyLevel: BehaviorSafetyLevelNormal,
	}
	goals := []Goal{
		{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive, Priority: GoalPriorityHigh},
	}
	intentions := []Intention{
		{GoalID: "g1", GoalType: GoalTypeConnection, Commitment: CommitmentStrong, Status: IntentionStatusExecuting},
	}
	result := checker.Verify(plan, goals, intentions)
	if !result.Consistent {
		t.Fatalf("对齐的计划应一致, 违规: %v", result.Violations)
	}
}

func TestConsistencyVerificationFailsOnBlockedSafety(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:     PlanVersionV1,
		ID:          "plan-2",
		CreatedAt:   time.Now().UTC(),
		Selected:    BehaviorCandidate{ID: "chat_reply", RiskScore: 0.95},
		SafetyLevel: BehaviorSafetyLevelBlocked,
	}
	result := checker.Verify(plan, nil, nil)
	if result.Consistent {
		t.Fatal("安全被阻止的计划应不一致")
	}
}

func TestConsistencyVerifyWithFallbackReturnsFallbackOnFailure(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:     PlanVersionV1,
		ID:          "plan-3",
		CreatedAt:   time.Now().UTC(),
		Selected:    BehaviorCandidate{ID: "chat_reply", RiskScore: 0.5},
		SafetyLevel: BehaviorSafetyLevelBlocked,
	}
	result := checker.VerifyWithFallback(plan, nil, nil)
	if result.Selected.ID != "wait_observe" {
		t.Fatalf("不一致应回退到 wait_observe, 实际 %s", result.Selected.ID)
	}
}

func TestConsistencyVerifyWithFallbackReturnsOriginalOnPass(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:     PlanVersionV1,
		ID:          "plan-4",
		CreatedAt:   time.Now().UTC(),
		Selected:    BehaviorCandidate{ID: "chat_reply", RiskScore: 0.1},
		SafetyLevel: BehaviorSafetyLevelNormal,
		DoNotSend:   false,
	}
	goals := []Goal{
		{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive, Priority: GoalPriorityHigh},
	}
	result := checker.VerifyWithFallback(plan, goals, nil)
	if result.ID != "plan-4" {
		t.Fatalf("一致时应返回原始计划, 实际 %s", result.ID)
	}
}

func TestCheckGoalConsistencyReturnsHalfWhenNoGoals(t *testing.T) {
	candidate := BehaviorCandidate{ID: "chat_reply"}
	score := checkGoalConsistency(candidate, nil)
	if score != 0.5 {
		t.Fatalf("无目标时应返回 0.5, 实际 %f", score)
	}
}

func TestCheckSafetyConsistencyDoNotSend(t *testing.T) {
	plan := BehaviorPlan{DoNotSend: true}
	score := checkSafetyConsistency(plan)
	if score != 1.0 {
		t.Fatalf("DoNotSend 时安全一致性应为 1.0, 实际 %f", score)
	}
}
