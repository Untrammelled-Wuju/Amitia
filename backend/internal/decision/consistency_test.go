package decision

import "testing"

func TestConsistencyPlanV2Valid(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:          PlanVersionV2,
		ID:               "plan:" + "test1",
		Selected:         BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2, RiskScore: 0.1},
		SafetyLevel:      BehaviorSafetyLevelNormal,
		NeedsExpression:  true,
		ExpressionPlanID: "expr:plan:test1",
		Audit:            BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
	}
	result := checker.Verify(plan)
	if !result.Consistent {
		t.Fatalf("有效的 V2 Plan 应一致, violations: %v", result.Violations)
	}
}

func TestConsistencyRejectsEmptyID(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:  PlanVersionV2,
		Selected: BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
		Audit:    BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("空 ID Plan 应不一致")
	}
}

func TestConsistencyRejectsWrongVersion(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:  PlanVersionV1,
		ID:       "plan:wrong",
		Selected: BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
		Audit:    BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("V1 Plan 应不一致")
	}
}

func TestConsistencyRejectsDoNotSendWithNeedsExpression(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:         PlanVersionV2,
		ID:              "plan:test",
		Selected:        BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
		Audit:           BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		DoNotSend:       true,
		NeedsExpression: true,
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("DoNotSend + NeedsExpression 应不一致")
	}
}

func TestConsistencyRejectsMissingExpressionPlanID(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:         PlanVersionV2,
		ID:              "plan:test",
		Selected:        BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
		Audit:           BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		NeedsExpression: true,
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("NeedsExpression 但无 ExpressionPlanID 应不一致")
	}
}

func TestConsistencyRejectsBlockedWithoutDoNotSend(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:     PlanVersionV2,
		ID:          "plan:test",
		Selected:    BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV2},
		Audit:       BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		SafetyLevel: BehaviorSafetyLevelBlocked,
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("Blocked 但无 DoNotSend 应不一致")
	}
}

func TestConsistencyRejectsDelayWithoutDoNotSend(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:     PlanVersionV2,
		ID:          "plan:test",
		Selected:    BehaviorCandidate{ID: "wait_observe", Tag: BehaviorTagDelay, ScoringVersion: BehaviorFormulaVersionV2},
		Audit:       BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		DoNotSend:   false,
		SafetyLevel: BehaviorSafetyLevelNormal,
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("delay tag 无 DoNotSend 应不一致")
	}
}

func TestConsistencyRejectsToolCallWithExpression(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:         PlanVersionV2,
		ID:              "plan:test",
		Selected:        BehaviorCandidate{ID: "tool_search", ActionType: CandidateActionToolCall, ScoringVersion: BehaviorFormulaVersionV2},
		Audit:           BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		NeedsExpression: true,
		SafetyLevel:     BehaviorSafetyLevelNormal,
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("tool_call 有 NeedsExpression 应不一致")
	}
}

func TestConsistencyRejectsScoringVersionMismatch(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version:          PlanVersionV2,
		ID:               "plan:test",
		Selected:         BehaviorCandidate{ID: "chat_reply", ScoringVersion: BehaviorFormulaVersionV1},
		Audit:            BehaviorAudit{FormulaVersion: BehaviorFormulaVersionV2},
		NeedsExpression:  true,
		ExpressionPlanID: "expr:plan:test",
		SafetyLevel:      BehaviorSafetyLevelNormal,
	}
	result := checker.Verify(plan)
	if result.Consistent {
		t.Fatal("ScoringVersion 不匹配应不一致")
	}
}

func TestConsistencyVerifyAndReportError(t *testing.T) {
	checker := DefaultConsistencyChecker()
	plan := BehaviorPlan{
		Version: PlanVersionV1,
		ID:      "plan:test",
	}
	_, err := checker.VerifyAndReport(plan)
	if err == nil {
		t.Fatal("VerifyAndReport 失败时应返回 error")
	}
}
