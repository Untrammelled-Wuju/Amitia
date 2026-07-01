package decision

import (
	"testing"
	"time"
)

func TestGenerateExpressionPlanBasic(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:        "plan-1",
		CreatedAt: now,
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
	exprPlan := GenerateExpressionPlan(input)
	if exprPlan.ExpressionType != ExpressionTypeText {
		t.Fatalf("chat_reply 应为 Text 表达类型, 实际 %s", exprPlan.ExpressionType)
	}
	if exprPlan.BehaviorPlanID != "plan-1" {
		t.Fatalf("关联 BehaviorPlanID 错误: %s", exprPlan.BehaviorPlanID)
	}
	if exprPlan.Suppressed {
		t.Fatal("安全场景不应被抑制")
	}
}

func TestGenerateExpressionPlanSafetyBlocked(t *testing.T) {
	now := time.Now().UTC()
	plan := BehaviorPlan{
		ID:        "plan-2",
		CreatedAt: now,
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
	exprPlan := GenerateExpressionPlan(input)
	if !exprPlan.SafetyBlocked {
		t.Fatal("安全阻止的表达式应标记 safetyBlocked")
	}
	if !exprPlan.DoNotSend {
		t.Fatal("安全阻止的表达式应标记 DoNotSend")
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
	tone := deriveExpressionTone(psyche, candidate)
	if tone != ExpressionToneFirm {
		t.Fatalf("SetBoundary 应为 Firm 语气, 实际 %s", tone)
	}
}
