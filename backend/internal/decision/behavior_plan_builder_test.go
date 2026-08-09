package decision

import (
	"testing"
	"time"
)

func TestNewBehaviorPlanIDUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewBehaviorPlanID()
		if ids[id] {
			t.Fatalf("Plan ID collision at iteration %d: %s", i, id)
		}
		ids[id] = true
	}
}

func TestExplicitPlanIDUsed(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		PlanID: "plan:test:123",
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ID != "plan:test:123" {
		t.Fatalf("应使用显式PlanID, got %s", plan.ID)
	}
	if plan.Version != PlanVersionV2 {
		t.Fatalf("版本应为 V2, got %s", plan.Version)
	}
}

func TestZeroNowError(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply"},
			HasSelection: true,
		},
	}
	_, err := builder.Build(input)
	if err == nil {
		t.Fatal("Zero Now 应返回 error")
	}
}

func TestNoSelectionReturnsNil(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{HasSelection: false},
		Now:         time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatal("No Selection 应返回 nil plan")
	}
}

func TestPriorityNotFromFinalScore(t *testing.T) {
	now := time.Now().UTC()
	goals := []Goal{
		{ID: "g1", Type: GoalTypeAutonomy, Status: GoalStatusActive, Priority: GoalPriorityHigh},
	}

	candidate1 := BehaviorCandidate{ID: "wait_observe", FinalScore: 0.9, Tag: BehaviorTagDelay, ScoringVersion: BehaviorFormulaVersionV2}
	candidate2 := BehaviorCandidate{ID: "set_boundary", FinalScore: 0.2, Tag: BehaviorTagSetBoundary, ScoringVersion: BehaviorFormulaVersionV2}

	builder := NewBehaviorPlanBuilder()
	input1 := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     candidate1,
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Goals: goals,
		Now:   now,
	}
	input2 := input1
	input2.Arbitration.Selected = candidate2
	input2.Arbitration.HasSelection = true

	plan1, err := builder.Build(input1)
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := builder.Build(input2)
	if err != nil {
		t.Fatal(err)
	}

	if plan1.Priority != plan2.Priority {
		t.Fatalf("不同FinalScore但同Goal时Priority应相同: %s vs %s", plan1.Priority, plan2.Priority)
	}
}

func TestPriorityFromGoalCritical(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	goals := []Goal{
		{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive, Priority: GoalPriorityCritical},
	}
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Goals: goals,
		Now:   time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Priority != BehaviorPriorityCritical {
		t.Fatalf("Critical Goal 应映射为 Critical Priority, got %s", plan.Priority)
	}
}

func TestWaitPriorityLow(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "wait_observe", Tag: BehaviorTagDelay, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Goals: nil,
		Now:   time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Priority != BehaviorPriorityLow {
		t.Fatalf("wait_observe 无匹配Goal时应为 Low, got %s", plan.Priority)
	}
}

func TestFallbackDisposition(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "wait_observe", Tag: BehaviorTagDelay, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionFallback,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SelectionDisposition != ArbitrationDispositionFallback {
		t.Fatalf("Disposition 应为 fallback, got %s", plan.SelectionDisposition)
	}
	if !plan.DoNotSend {
		t.Fatal("Fallback wait_observe 应为 DoNotSend")
	}
	if plan.NeedsExpression {
		t.Fatal("Fallback wait_observe 不应 NeedsExpression")
	}
}

func TestWaitObserveDoNotSend(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "wait_observe", Tag: BehaviorTagDelay, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.DoNotSend {
		t.Fatal("wait_observe 应为 DoNotSend")
	}
	if plan.NeedsExpression {
		t.Fatal("wait_observe 不应 NeedsExpression")
	}
	if plan.ExpressionPlanID != "" {
		t.Fatal("wait_observe 不应有 ExpressionPlanID")
	}
}

func TestToolCallDoNotSend(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "tool_search", ActionType: CandidateActionToolCall, Tag: BehaviorTag("tool"), ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.DoNotSend {
		t.Fatal("tool_call 应为 DoNotSend")
	}
	if plan.NeedsExpression {
		t.Fatal("tool_call 不应 NeedsExpression")
	}
}

func TestChatReplyNeedsExpression(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", Tag: BehaviorTagReply, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.DoNotSend {
		t.Fatal("chat_reply 应为 DoNotSend=false")
	}
	if !plan.NeedsExpression {
		t.Fatal("chat_reply 应 NeedsExpression")
	}
	if plan.ExpressionPlanID == "" {
		t.Fatal("chat_reply 应有 ExpressionPlanID")
	}
	if plan.ExpressionPlanID != "expr:"+plan.ID {
		t.Fatalf("ExpressionPlanID 格式错误: %s", plan.ExpressionPlanID)
	}
}

func TestSafetyBlockedOverride(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", Tag: BehaviorTagReply, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Safety: PlanSafetyContext{Blocked: true, Level: BehaviorSafetyLevelBlocked},
		Now:    time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SafetyLevel != BehaviorSafetyLevelBlocked {
		t.Fatal("SafetyLevel 应为 blocked")
	}
	if !plan.DoNotSend {
		t.Fatal("Safety Block 应导致 DoNotSend")
	}
	if plan.NeedsExpression {
		t.Fatal("Safety Block 不应 NeedsExpression")
	}
}

func TestHighRiskConservative(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", Tag: BehaviorTagReply, RiskScore: 0.85, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Safety: PlanSafetyContext{Level: BehaviorSafetyLevelNormal, Blocked: false},
		Now:    time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SafetyLevel == BehaviorSafetyLevelBlocked {
		t.Fatal("High Risk 不应直接 Blocked")
	}
}

func TestPlanClonesInput(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	relDims := map[RelationshipDimension]RelationshipDimensionValue{
		RelationshipFamiliarity: {Value: 0.5},
	}
	rels := make([]BehaviorReason, 0)
	cand := BehaviorCandidate{
		ID:             "chat_reply",
		Tag:            BehaviorTagReply,
		Reasons:        rels,
		BaseScore:      0.6,
		ScoringVersion: BehaviorFormulaVersionV2,
	}
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     cand,
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit: BehaviorAudit{
				FormulaVersion: BehaviorFormulaVersionV2,
				Diagnostics:    []string{"original-diag"},
			},
		},
		Relationship: RelationshipSnapshot{Dimensions: relDims},
		Now:          time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Selected.Reasons != nil {
		reasons := plan.Selected.Reasons
		reasons = append(reasons, BehaviorReason{Source: "mutation"})
		if len(input.Arbitration.Selected.Reasons) != 0 {
			t.Fatal("修改 Plan Reasons 不应影响 Input")
		}
	}
	relDims[RelationshipFamiliarity] = RelationshipDimensionValue{Value: 9.9}
	if plan.Relationship.Dimensions[RelationshipFamiliarity].Value == 9.9 {
		t.Fatal("修改原 Relationship 不应影响 Plan")
	}
	if len(input.Arbitration.Audit.Diagnostics) != 1 {
		t.Fatal("Diagnostics 应被 clone")
	}
}

func TestIntentIsStableToken(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	tests := []BehaviorTag{BehaviorTagReply, BehaviorTagAskClarify, BehaviorTagOfferSupport, BehaviorTagSetBoundary, BehaviorTagRepair, BehaviorTagProactiveCheck, BehaviorTagDelay}
	for _, tag := range tests {
		input := BehaviorPlanBuildInput{
			Arbitration: ArbitrationResult{
				Selected:     BehaviorCandidate{ID: "c", Tag: tag, ScoringVersion: BehaviorFormulaVersionV2},
				HasSelection: true,
				Disposition:  ArbitrationDispositionSelected,
				Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
			},
			Now: time.Now().UTC(),
		}
		plan, err := builder.Build(input)
		if err != nil {
			t.Fatal(err)
		}
		if string(plan.Intent) == "" || string(plan.Strategy) == "" {
			t.Fatalf("Tag %s: Intent/Strategy 不能为空", tag)
		}
	}
}

func TestResponseGoalToken(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", Tag: BehaviorTagReply, ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit:        BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ResponseGoal == "" {
		t.Fatal("ResponseGoal 不能为空")
	}
}

func TestAuditPropagation(t *testing.T) {
	builder := NewBehaviorPlanBuilder()
	input := BehaviorPlanBuildInput{
		Arbitration: ArbitrationResult{
			Selected:     BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
			HasSelection: true,
			Disposition:  ArbitrationDispositionSelected,
			Audit: BehaviorAudit{
				FormulaVersion: BehaviorFormulaVersionV2,
				ConflictIDs:    []string{"conflict:x>y"},
				Diagnostics:    []string{"arb-diag"},
			},
		},
		Now: time.Now().UTC(),
	}
	plan, err := builder.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Audit.FormulaVersion != BehaviorFormulaVersionV2 {
		t.Fatal("FormulaVersion 应传递")
	}
	hasSelectedDiag := false
	for _, d := range plan.Audit.Diagnostics {
		if d == "plan:selected:chat_reply" {
			hasSelectedDiag = true
		}
	}
	if !hasSelectedDiag {
		t.Fatal("应添加 plan:selected 诊断")
	}
}
