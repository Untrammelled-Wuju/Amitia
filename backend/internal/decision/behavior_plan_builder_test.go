package decision

import (
	"testing"
	"time"
)

func TestDerivePlanPriorityCritical(t *testing.T) {
	candidate := BehaviorCandidate{FinalScore: 0.85}
	priority := derivePlanPriority(candidate)
	if priority != BehaviorPriorityCritical {
		t.Fatalf("0.85分应为 Critical, 实际 %s", priority)
	}
}

func TestDerivePlanPriorityLow(t *testing.T) {
	candidate := BehaviorCandidate{FinalScore: 0.1}
	priority := derivePlanPriority(candidate)
	if priority != BehaviorPriorityLow {
		t.Fatalf("0.1分应为 Low, 实际 %s", priority)
	}
}

func TestDerivePlanSafetyLevelBlocked(t *testing.T) {
	candidate := BehaviorCandidate{RiskScore: 0.9}
	level := derivePlanSafety(candidate)
	if level != BehaviorSafetyLevelBlocked {
		t.Fatalf("0.9风险分应为 Blocked, 实际 %s", level)
	}
}

func TestNewBehaviorPlanBuilder(t *testing.T) {
	now := time.Now().UTC()
	builder := NewBehaviorPlanBuilder(now)
	candidate := BehaviorCandidate{ID: "chat_reply", FinalScore: 0.7}
	input := ArbitrationInput{}
	plan := builder.Build(candidate, input)
	if plan.Version != PlanVersionV1 {
		t.Fatalf("版本应为 V1, 实际 %s", plan.Version)
	}
	if plan.ID == "" {
		t.Fatal("Plan ID 不能为空")
	}
	if plan.Selected.ID != "chat_reply" {
		t.Fatalf("应包含选中的候选, 实际 %s", plan.Selected.ID)
	}
	if plan.Priority != BehaviorPriorityHigh {
		t.Fatalf("0.7分应为 High, 实际 %s", plan.Priority)
	}
}
