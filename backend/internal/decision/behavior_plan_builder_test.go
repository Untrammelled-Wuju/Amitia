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
	candidate := BehaviorCandidate{ID: "chat_reply", FinalScore: 0.7, Tag: BehaviorTagReply}
	input := ArbitrationInput{
		Psyche: PsycheSignalSet{
			Mood:   ScalarSignal{Value: 0.6},
			Stress: ScalarSignal{Value: 0.2},
		},
		Relationship: RelationshipSnapshot{
			Dimensions: map[RelationshipDimension]RelationshipDimensionValue{
				RelationshipFamiliarity: {Value: 0.5},
			},
		},
	}
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
	if plan.Intent != "正常回复" {
		t.Fatalf("Intent 应为 正常回复, 实际 %s", plan.Intent)
	}
	if plan.Strategy != "自然回应，保持对话流畅" {
		t.Fatalf("Strategy 应为 自然回应，保持对话流畅, 实际 %s", plan.Strategy)
	}
	if len(plan.AllowedTopics) == 0 {
		t.Fatal("AllowedTopics 不能为空")
	}
	if len(plan.ForbiddenTopics) == 0 {
		t.Fatal("ForbiddenTopics 不能为空")
	}
	if plan.ResponseGoal == "" {
		t.Fatal("ResponseGoal 不能为空")
	}
	if plan.ToneHint == "" {
		t.Fatal("ToneHint 不能为空")
	}
	if plan.Priority != BehaviorPriorityHigh {
		t.Fatalf("0.7分应为 High, 实际 %s", plan.Priority)
	}
}

func TestDerivePlanIntentOffersSupport(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagOfferSupport}
	intent := derivePlanIntent(candidate, ArbitrationInput{})
	if intent != "提供支持" {
		t.Fatalf("OfferSupport 意图应为 提供支持, 实际 %s", intent)
	}
}

func TestDerivePlanIntentSetBoundary(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagSetBoundary}
	intent := derivePlanIntent(candidate, ArbitrationInput{})
	if intent != "设立边界" {
		t.Fatalf("SetBoundary 意图应为 设立边界, 实际 %s", intent)
	}
}

func TestDerivePlanIntentRepair(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagRepair}
	intent := derivePlanIntent(candidate, ArbitrationInput{})
	if intent != "关系修复" {
		t.Fatalf("Repair 意图应为 关系修复, 实际 %s", intent)
	}
}

func TestDerivePlanIntentProactiveCheck(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagProactiveCheck}
	intent := derivePlanIntent(candidate, ArbitrationInput{})
	if intent != "主动关心" {
		t.Fatalf("ProactiveCheck 意图应为 主动关心, 实际 %s", intent)
	}
}

func TestDerivePlanIntentAskClarify(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagAskClarify}
	intent := derivePlanIntent(candidate, ArbitrationInput{})
	if intent != "请求澄清" {
		t.Fatalf("AskClarify 意图应为 请求澄清, 实际 %s", intent)
	}
}

func TestDerivePlanIntentDelay(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagDelay}
	intent := derivePlanIntent(candidate, ArbitrationInput{})
	if intent != "延迟观察" {
		t.Fatalf("Delay 意图应为 延迟观察, 实际 %s", intent)
	}
}

func TestDerivePlanForbiddenTopicsHighRisk(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagReply, RiskScore: 0.8}
	topics := derivePlanForbiddenTopics(candidate, ArbitrationInput{})
	found := false
	for _, t := range topics {
		if t == "高风险话题" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("高风险候选应有高风险话题禁止, 实际: %v", topics)
	}
}

func TestDerivePlanToneHintBoundaryTag(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagSetBoundary}
	input := ArbitrationInput{
		Psyche: PsycheSignalSet{
			Mood:   ScalarSignal{Value: 0.5},
			Stress: ScalarSignal{Value: 0.3},
		},
	}
	hint := derivePlanToneHint(candidate, input)
	if hint != "坚定" {
		t.Fatalf("SetBoundary 语气应为 坚定, 实际 %s", hint)
	}
}

func TestDerivePlanToneHintHighStress(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagReply}
	input := ArbitrationInput{
		Psyche: PsycheSignalSet{
			Mood:   ScalarSignal{Value: 0.4},
			Stress: ScalarSignal{Value: 0.85},
		},
	}
	hint := derivePlanToneHint(candidate, input)
	if hint != "轻柔克制" {
		t.Fatalf("高压力语气应为 轻柔克制, 实际 %s", hint)
	}
}

func TestDerivePlanToneHintLowMood(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagReply}
	input := ArbitrationInput{
		Psyche: PsycheSignalSet{
			Mood:   ScalarSignal{Value: 0.2},
			Stress: ScalarSignal{Value: 0.3},
		},
	}
	hint := derivePlanToneHint(candidate, input)
	if hint != "温暖安抚" {
		t.Fatalf("情绪低落语气应为 温暖安抚, 实际 %s", hint)
	}
}

func TestDerivePlanStrategyDifferentTags(t *testing.T) {
	tags := []BehaviorTag{
		BehaviorTagReply, BehaviorTagAskClarify, BehaviorTagOfferSupport,
		BehaviorTagSetBoundary, BehaviorTagRepair, BehaviorTagProactiveCheck, BehaviorTagDelay,
	}
	for _, tag := range tags {
		candidate := BehaviorCandidate{Tag: tag}
		strategy := derivePlanStrategy(candidate, ArbitrationInput{})
		if strategy == "" {
			t.Fatalf("Tag %s 的策略不能为空", tag)
		}
		if strategy == "保持自然沟通" && tag != BehaviorTagReply {
			continue
		}
	}
}

func TestDerivePlanAllowedTopicsReply(t *testing.T) {
	candidate := BehaviorCandidate{Tag: BehaviorTagReply}
	topics := derivePlanAllowedTopics(candidate, ArbitrationInput{})
	if len(topics) < 2 {
		t.Fatalf("Reply 允许话题数量不足: %v", topics)
	}
}

func TestDerivePlanResponseGoalDifferentTags(t *testing.T) {
	tags := []BehaviorTag{
		BehaviorTagReply, BehaviorTagAskClarify, BehaviorTagOfferSupport,
		BehaviorTagSetBoundary, BehaviorTagRepair, BehaviorTagProactiveCheck, BehaviorTagDelay,
	}
	for _, tag := range tags {
		candidate := BehaviorCandidate{Tag: tag}
		goal := derivePlanResponseGoal(candidate, ArbitrationInput{})
		if goal == "" {
			t.Fatalf("Tag %s 的回复目标不能为空", tag)
		}
	}
}
