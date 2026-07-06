package chat

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/interaction"
)

func TestBuildBehaviorPlanFromRuntimeWithPlan(t *testing.T) {
	bp := &decision.BehaviorPlan{
		Intent:          "正常回复",
		Strategy:        "自然回应，保持对话流畅",
		AllowedTopics:   []string{"日常对话", "感受表达", "开放式交流", "当前话题延伸"},
		ForbiddenTopics: []string{"不适当的亲密关系请求", "违法违规内容"},
		ResponseGoal:    "让对话自然流畅地继续",
		ToneHint:        "自然友好",
		Priority:        decision.BehaviorPriorityNormal,
		SafetyLevel:     decision.BehaviorSafetyLevelNormal,
	}

	runtime := &interaction.RuntimeAssembly{
		BehaviorPlan: bp,
		Path:         interaction.PathTypeDeep,
	}
	prompt := buildBehaviorPlanFromRuntime(runtime)
	if !strings.Contains(prompt, "意图: 正常回复") {
		t.Fatalf("missing intent: %s", prompt)
	}
	if !strings.Contains(prompt, "策略: 自然回应") {
		t.Fatalf("missing strategy: %s", prompt)
	}
	if !strings.Contains(prompt, "允许话题:") {
		t.Fatalf("missing allowed_topics: %s", prompt)
	}
	if !strings.Contains(prompt, "禁止话题:") {
		t.Fatalf("missing forbidden_topics: %s", prompt)
	}
	if !strings.Contains(prompt, "回复目标:") {
		t.Fatalf("missing response_goal: %s", prompt)
	}
	if !strings.Contains(prompt, "语气提示:") {
		t.Fatalf("missing tone_hint: %s", prompt)
	}
	if strings.Contains(prompt, "{") || strings.Contains(prompt, "}") {
		t.Fatalf("should not contain JSON: %s", prompt)
	}
}

func TestBuildBehaviorPlanFromRuntimeNilPlan(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{
		BehaviorPlan: nil,
		Path:         interaction.PathTypeDeep,
		Safety: interaction.RuntimeSafetyDecision{
			Reasons: []string{"high_stress", "relationship_tension"},
		},
	}
	prompt := buildBehaviorPlanFromRuntime(runtime)
	if !strings.Contains(prompt, "路径: deep") {
		t.Fatalf("missing path in fallback: %s", prompt)
	}
	if !strings.Contains(prompt, "安全因素:") {
		t.Fatalf("missing safety reasons: %s", prompt)
	}
}

func TestBuildBehaviorPlanFromRuntimeNilRuntime(t *testing.T) {
	prompt := buildBehaviorPlanFromRuntime(nil)
	if prompt != "" {
		t.Fatalf("nil runtime should return empty: %s", prompt)
	}
}
