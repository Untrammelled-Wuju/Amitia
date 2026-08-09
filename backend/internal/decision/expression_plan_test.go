package decision

import (
	"testing"
	"time"
)

func TestGenerateExpressionPlanBasic(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:               "plan-1",
		ExpressionPlanID: "expr:plan-1",
		CreatedAt:        now,
		NeedsExpression:  true,
		Selected: BehaviorCandidate{
			ID:      "chat_reply",
			Tag:     BehaviorTagReply,
			Channel: BehaviorChannelChat,
		},
	}
	psyche := PsycheSignalSet{
		Stress: ScalarSignal{Value: 0.3},
	}
	exprCtrl := ExpressionControlInput{
		EmotionIntensity:   0.4,
		RiskScore:          0.1,
		RelationshipSafety: 0.8,
		StressLevel:        0.2,
	}
	input := ExpressionPlanInput{
		BehaviorPlan:   plan,
		Psyche:         psyche,
		ExpressionCtrl: exprCtrl,
		CopingStrategy: CopingReappraisal,
		SafetyResult:   SafetyCheckResult{Passed: true, Blocked: false},
		Now:            now,
	}
	exprPlan, err := GenerateExpressionPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if exprPlan.ExpressionType != ExpressionTypeText {
		t.Fatalf("chat_reply 应为 Text 表达类型, 实际 %s", exprPlan.ExpressionType)
	}
	if exprPlan.BehaviorPlanID != "plan-1" {
		t.Fatalf("关联 BehaviorPlanID 错误: %s", exprPlan.BehaviorPlanID)
	}
	if exprPlan.ID != "expr:plan-1" {
		t.Fatalf("ExpressionPlan ID 错误: %s", exprPlan.ID)
	}
	if exprPlan.Suppressed {
		t.Fatal("安全场景不应被抑制")
	}
}

func TestGenerateExpressionPlanSafetyBlocked(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:               "plan-2",
		ExpressionPlanID: "expr:plan-2",
		CreatedAt:        now,
		NeedsExpression:  true,
		Selected: BehaviorCandidate{
			ID:      "chat_reply",
			Tag:     BehaviorTagReply,
			Channel: BehaviorChannelChat,
		},
	}
	psyche := PsycheSignalSet{}
	exprCtrl := ExpressionControlInput{
		EmotionIntensity:   0.5,
		RiskScore:          0.2,
		RelationshipSafety: 0.8,
	}
	input := ExpressionPlanInput{
		BehaviorPlan:   plan,
		Psyche:         psyche,
		ExpressionCtrl: exprCtrl,
		CopingStrategy: CopingReappraisal,
		SafetyResult:   SafetyCheckResult{Passed: false, Blocked: true, Reason: "blocked_phrase"},
		Now:            now,
	}
	exprPlan, err := GenerateExpressionPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if !exprPlan.SafetyBlocked {
		t.Fatal("安全阻止的表达式应标记 safetyBlocked")
	}
	if !exprPlan.DoNotSend {
		t.Fatal("安全阻止的表达式应标记 DoNotSend")
	}
}

func TestGenerateExpressionPlanRejectsDoNotSend(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:               "plan-3",
		ExpressionPlanID: "expr:plan-3",
		CreatedAt:        now,
		NeedsExpression:  true,
		DoNotSend:        true,
		Selected: BehaviorCandidate{
			ID:      "wait_observe",
			Tag:     BehaviorTagDelay,
			Channel: BehaviorChannelSystem,
		},
	}
	input := ExpressionPlanInput{
		BehaviorPlan: plan,
		Psyche:       PsycheSignalSet{},
		SafetyResult: SafetyCheckResult{Passed: true},
		Now:          now,
	}
	_, err := GenerateExpressionPlan(input)
	if err == nil {
		t.Fatal("DoNotSend=true 应返回 error")
	}
}

func TestGenerateExpressionPlanRejectsMissingPlanID(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:              "",
		NeedsExpression: true,
		Selected:        BehaviorCandidate{ID: "chat_reply"},
	}
	input := ExpressionPlanInput{
		BehaviorPlan: plan,
		Now:          now,
	}
	_, err := GenerateExpressionPlan(input)
	if err == nil {
		t.Fatal("空 Plan ID 应返回 error")
	}
}

func TestGenerateExpressionPlanRejectsMissingExpressionPlanID(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:               "plan-4",
		NeedsExpression:  true,
		ExpressionPlanID: "",
		Selected:         BehaviorCandidate{ID: "chat_reply"},
	}
	input := ExpressionPlanInput{
		BehaviorPlan: plan,
		Now:          now,
	}
	_, err := GenerateExpressionPlan(input)
	if err == nil {
		t.Fatal("空 ExpressionPlanID 应返回 error")
	}
}

func TestGenerateExpressionPlanRejectsZeroNow(t *testing.T) {
	plan := BehaviorPlan{
		ID:               "plan-5",
		NeedsExpression:  true,
		ExpressionPlanID: "expr:plan-5",
		Selected:         BehaviorCandidate{ID: "chat_reply"},
	}
	input := ExpressionPlanInput{BehaviorPlan: plan}
	_, err := GenerateExpressionPlan(input)
	if err == nil {
		t.Fatal("Zero Now 应返回 error")
	}
}

func TestGenerateExpressionPlanRejectsNoExpressionNeeded(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:               "plan-6",
		NeedsExpression:  false,
		ExpressionPlanID: "expr:plan-6",
		Selected:         BehaviorCandidate{ID: "chat_reply"},
	}
	input := ExpressionPlanInput{
		BehaviorPlan: plan,
		Now:          now,
	}
	_, err := GenerateExpressionPlan(input)
	if err == nil {
		t.Fatal("NeedsExpression=false 应返回 error")
	}
}

func TestMapExpressionType(t *testing.T) {
	if mapExpressionType(BehaviorTagAskClarify) != ExpressionTypeQuestion {
		t.Fatal("AskClarify 应为 Question 类型")
	}
	if mapExpressionType(BehaviorTagSetBoundary) != ExpressionTypeBoundary {
		t.Fatal("SetBoundary 应为 Boundary 类型")
	}
	if mapExpressionType(BehaviorTagDelay) != ExpressionTypeSilence {
		t.Fatal("Delay 应为 Silence 类型")
	}
}

func TestDeriveExpressionToneSetBoundary(t *testing.T) {
	psyche := PsycheSignalSet{}
	candidate := BehaviorCandidate{Tag: BehaviorTagSetBoundary}
	tone := deriveExpressionTone(psyche, candidate, nil)
	if tone != ExpressionToneFirm {
		t.Fatalf("SetBoundary 应为 Firm 语气, 实际 %s", tone)
	}
}
