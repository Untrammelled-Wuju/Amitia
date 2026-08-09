package decision

import (
	"strings"
	"testing"
	"time"
)

func TestBuildActionDirectiveReturnsZeroValueForNilPlan(t *testing.T) {
	directive, err := BuildActionDirective(nil)
	if err != nil {
		t.Fatalf("nil plan should not error: %v", err)
	}
	if directive.PlanID != "" || directive.Kind != "" {
		t.Fatalf("nil plan should return zero directive, got %+v", directive)
	}
}

func TestBuildActionDirectiveRejectsEmptyPlanID(t *testing.T) {
	plan := &BehaviorPlan{
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "chat_reply", ActionType: CandidateActionChat},
	}
	_, err := BuildActionDirective(plan)
	if err == nil {
		t.Fatal("expected error for empty plan ID")
	}
	if !strings.Contains(err.Error(), "PLAN_INVALID") {
		t.Fatalf("expected PLAN_INVALID, got: %v", err)
	}
}

func TestBuildActionDirectiveRejectsNonV2Plan(t *testing.T) {
	plan := &BehaviorPlan{
		ID:       "plan-1",
		Version:  PlanVersionV1,
		Selected: BehaviorCandidate{ID: "chat_reply", ActionType: CandidateActionChat},
	}
	_, err := BuildActionDirective(plan)
	if err == nil {
		t.Fatal("expected error for non-V2 plan")
	}
}

func TestBuildActionDirectiveMapsChatReplyToRespond(t *testing.T) {
	plan := &BehaviorPlan{
		ID:       "plan-1",
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "chat_reply", ActionType: CandidateActionChat, Tag: BehaviorTagReply},
	}
	directive, err := BuildActionDirective(plan)
	if err != nil {
		t.Fatal(err)
	}
	if directive.Kind != ActionDirectiveRespond {
		t.Fatalf("chat_reply should map to respond, got %s", directive.Kind)
	}
	if directive.Required {
		t.Fatal("respond directive should not be required")
	}
	if directive.PlanID != "plan-1" || directive.CandidateID != "chat_reply" {
		t.Fatalf("directive ids wrong: %+v", directive)
	}
}

func TestBuildActionDirectiveMapsWaitToWait(t *testing.T) {
	plan := &BehaviorPlan{
		ID:       "plan-2",
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "wait_observe", ActionType: CandidateActionWait, Tag: BehaviorTagDelay},
	}
	directive, err := BuildActionDirective(plan)
	if err != nil {
		t.Fatal(err)
	}
	if directive.Kind != ActionDirectiveWait {
		t.Fatalf("wait_observe should map to wait, got %s", directive.Kind)
	}
}

func TestBuildActionDirectiveMapsToolCallToTool(t *testing.T) {
	plan := &BehaviorPlan{
		ID:       "plan-3",
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "tool_search", ActionType: CandidateActionToolCall},
	}
	directive, err := BuildActionDirective(plan)
	if err != nil {
		t.Fatal(err)
	}
	if directive.Kind != ActionDirectiveTool {
		t.Fatalf("tool_call should map to tool, got %s", directive.Kind)
	}
	if !directive.Required {
		t.Fatal("tool directive should be required")
	}
}

func TestBuildActionDirectiveBlockedSafetyProducesWait(t *testing.T) {
	plan := &BehaviorPlan{
		ID:          "plan-4",
		Version:     PlanVersionV2,
		SafetyLevel: BehaviorSafetyLevelBlocked,
		Selected:    BehaviorCandidate{ID: "tool_search", ActionType: CandidateActionToolCall},
	}
	directive, err := BuildActionDirective(plan)
	if err != nil {
		t.Fatal(err)
	}
	if directive.Kind != ActionDirectiveWait {
		t.Fatalf("blocked safety should produce wait, got %s", directive.Kind)
	}
	if directive.Required {
		t.Fatal("blocked directive should not be required")
	}
}

func TestBuildActionDirectivePreservesIntentStrategy(t *testing.T) {
	plan := &BehaviorPlan{
		ID:       "plan-5",
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "ask_clarify", ActionType: CandidateActionAskClarify},
		Intent:   PlanIntentClarify,
		Strategy: StrategyRequestClarification,
	}
	directive, err := BuildActionDirective(plan)
	if err != nil {
		t.Fatal(err)
	}
	if directive.Intent != PlanIntentClarify {
		t.Fatalf("intent not preserved: %s", directive.Intent)
	}
	if directive.Strategy != StrategyRequestClarification {
		t.Fatalf("strategy not preserved: %s", directive.Strategy)
	}
}

func TestActionMaterializationErrorCodeImplementsError(t *testing.T) {
	var err error = ErrActionPlanInvalid
	if err.Error() != "ACTION_PLAN_INVALID" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}
}

func TestBuildActionDirectiveTimestamp(t *testing.T) {
	_ = time.Now()
	plan := &BehaviorPlan{
		ID:       "plan-ts",
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "chat_reply", ActionType: CandidateActionChat},
	}
	directive, err := BuildActionDirective(plan)
	if err != nil {
		t.Fatal(err)
	}
	if directive.PlanID != "plan-ts" {
		t.Fatal("directive plan id mismatch")
	}
}
