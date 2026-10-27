package safety

import (
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestBuildBdiRuntimeNormalizesLegacyCoping(t *testing.T) {
	policy := DefaultPolicy()
	policy.BDI.CopingStrategy.Selected = "reframe"
	runtime := BuildBdiRuntime(policy)
	if runtime.Coping != decision.CopingReappraisal {
		t.Fatalf("expected reappraisal, got %s", runtime.Coping)
	}
	if len(runtime.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", runtime.Warnings)
	}
}

func TestBuildBdiRuntimeAppliesConstraints(t *testing.T) {
	policy := DefaultPolicy()
	policy.BDI.HardConstraints = []HardConstraint{
		{RuleID: "r1", CandidateKey: "proactive_greet", Reason: "前任话题", Severity: "block"},
		{RuleID: "r2", CandidateKey: "offer_support", Reason: "先安抚", Severity: "override"},
		{RuleID: "r3", CandidateKey: "not_exists", Reason: "x", Severity: "block"},
	}
	runtime := BuildBdiRuntime(policy)
	if len(runtime.Filter.BlockedIDs) != 1 || runtime.Filter.BlockedIDs[0] != "proactive_greet" {
		t.Fatalf("unexpected blocked ids: %v", runtime.Filter.BlockedIDs)
	}
	if len(runtime.OverrideIDs) != 1 || runtime.OverrideIDs[0] != "offer_support" {
		t.Fatalf("unexpected override ids: %v", runtime.OverrideIDs)
	}
	if len(runtime.ForbiddenTopics) != 2 {
		t.Fatalf("unexpected forbidden topics: %v", runtime.ForbiddenTopics)
	}
	if len(runtime.Warnings) != 1 {
		t.Fatalf("expected one warning, got %v", runtime.Warnings)
	}
}

func TestBuildBdiRuntimeComputesSoftPreference(t *testing.T) {
	policy := DefaultPolicy()
	policy.BDI.SoftPreferences = []SoftPreference{{Dimension: "chat_reply", RawScore: 9, NormalizedWeight: 0.5}}
	runtime := BuildBdiRuntime(policy)
	value, ok := runtime.SoftPreferences["chat_reply"]
	if !ok {
		t.Fatalf("missing preference")
	}
	if value < 0.39 || value > 0.41 {
		t.Fatalf("unexpected preference value: %f", value)
	}
}

func TestApplyEmotionDirectiveCaps(t *testing.T) {
	policy := DefaultPolicy()
	policy.Config.NegativeEmotionCap = 4
	policy.BDI.EmotionExpression.DisplayMode = "show"
	policy.BDI.EmotionExpression.DisplayIntensity = 10
	runtime := BuildBdiRuntime(policy)
	plan := decision.ExpressionPlan{Tone: decision.ExpressionToneConcerned, EmotionIntensity: 0.9, Metadata: map[string]any{}}
	plan = ApplyEmotionDirective(plan, runtime.Emotion)
	if plan.EmotionIntensity > 0.4001 {
		t.Fatalf("negative cap not applied: %f", plan.EmotionIntensity)
	}
	if capped, _ := plan.Metadata["negativeEmotionCapped"].(bool); !capped {
		t.Fatalf("expected cap flag")
	}

	suppress := decision.ExpressionPlan{Tone: decision.ExpressionToneWarm, EmotionIntensity: 0.9, Metadata: map[string]any{}}
	policy.BDI.EmotionExpression.DisplayMode = "suppress"
	policy.BDI.EmotionExpression.DisplayIntensity = 3
	runtime = BuildBdiRuntime(policy)
	suppress = ApplyEmotionDirective(suppress, runtime.Emotion)
	if !suppress.Suppressed {
		t.Fatalf("expected suppressed plan")
	}
	if suppress.EmotionIntensity > 0.3001 {
		t.Fatalf("display intensity not applied: %f", suppress.EmotionIntensity)
	}

	policy.BDI.EmotionExpression.DisplayMode = "show"
	policy.BDI.EmotionExpression.DisplayIntensity = 0
	runtime = BuildBdiRuntime(policy)
	hidden := ApplyEmotionDirective(decision.ExpressionPlan{Tone: decision.ExpressionToneNeutral, EmotionIntensity: 0.9, Metadata: map[string]any{}}, runtime.Emotion)
	if !hidden.Suppressed || hidden.EmotionIntensity != 0 {
		t.Fatalf("zero display intensity must suppress expression: %+v", hidden)
	}
}

func TestValidateBdiConfigRejectsUnknownValues(t *testing.T) {
	config := DefaultBdiConfig()
	config.HardConstraints = []HardConstraint{{RuleID: "r1", CandidateKey: "unknown", Reason: "", Severity: "block"}}
	if err := ValidateBdiConfig(config); err == nil {
		t.Fatalf("expected validation error")
	}
	config = DefaultBdiConfig()
	config.CopingStrategy.Selected = "unknown"
	if err := ValidateBdiConfig(config); err == nil {
		t.Fatalf("expected coping validation error")
	}
	config = DefaultBdiConfig()
	config.HardConstraints = []HardConstraint{{RuleID: "r1", CandidateKey: "chat_reply", Reason: "x", Severity: "unknown"}}
	if err := ValidateBdiConfig(config); err == nil {
		t.Fatalf("expected severity validation error")
	}
	config = DefaultBdiConfig()
	config.EmotionExpression.DisplayMode = "unknown"
	if err := ValidateBdiConfig(config); err == nil {
		t.Fatalf("expected display mode validation error")
	}
}

func TestNormalizeBdiConfigRewritesLegacyValues(t *testing.T) {
	config := NormalizeBdiConfig(BdiConfig{
		HardConstraints: []HardConstraint{{RuleID: " r1 ", CandidateKey: " chat_reply ", Severity: ""}},
		CopingStrategy:  &CopingStrategy{Selected: "support", Alternatives: []string{"avoid"}},
	})
	if config.HardConstraints[0].Severity != ConstraintSeverityBlock || config.HardConstraints[0].RuleID != "r1" {
		t.Fatalf("unexpected rule: %+v", config.HardConstraints[0])
	}
	if config.CopingStrategy.Selected != string(decision.CopingSeekSupport) || config.CopingStrategy.Alternatives[0] != string(decision.CopingAvoidance) {
		t.Fatalf("unexpected coping: %+v", config.CopingStrategy)
	}
}
