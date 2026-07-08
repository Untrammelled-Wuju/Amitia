package chat

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/interaction"
)

func TestDeriveEmotionLabel_ToneMapping(t *testing.T) {
	tests := []struct {
		name          string
		tone          decision.ExpressionTone
		intensity     float64
		suppressed    bool
		expectedLabel string
	}{
		{"warm high intensity", decision.ExpressionToneWarm, 0.8, false, "SWEET_ATTACHMENT"},
		{"warm low intensity", decision.ExpressionToneWarm, 0.3, false, "QUIET_FOND"},
		{"neutral", decision.ExpressionToneNeutral, 0.5, false, "CALM_RATIONAL"},
		{"playful", decision.ExpressionTonePlayful, 0.5, false, "TSUNDERE"},
		{"concerned high", decision.ExpressionToneConcerned, 0.7, false, "HURT_GRIEVANCE"},
		{"concerned low", decision.ExpressionToneConcerned, 0.3, false, "FEARFUL_OBEDIENT"},
		{"firm", decision.ExpressionToneFirm, 0.5, false, "ANGRY_ATTACK"},
		{"soft suppressed", decision.ExpressionToneSoft, 0.5, true, "COLD_DETACHED"},
		{"soft not suppressed", decision.ExpressionToneSoft, 0.5, false, "SHY_HEARTBEAT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveEmotionLabel(tt.tone, tt.intensity, tt.suppressed)
			if got != tt.expectedLabel {
				t.Errorf("expected %s, got %s", tt.expectedLabel, got)
			}
		})
	}
}

func TestDeriveEmotionLabel_DifferentTonesProduceDifferentLabels(t *testing.T) {
	warm := deriveEmotionLabel(decision.ExpressionToneWarm, 0.8, false)
	firm := deriveEmotionLabel(decision.ExpressionToneFirm, 0.5, false)
	neutral := deriveEmotionLabel(decision.ExpressionToneNeutral, 0.5, false)

	if warm == firm {
		t.Error("warm and firm should produce different labels")
	}
	if warm == neutral {
		t.Error("warm and neutral should produce different labels")
	}
	if firm == neutral {
		t.Error("firm and neutral should produce different labels")
	}
}

func TestBuildEmotionFusionRaw_NilRuntime(t *testing.T) {
	result := buildEmotionFusionRaw(nil, "Amitia")
	if result != "" {
		t.Error("nil runtime should return empty string")
	}
}

func TestBuildEmotionFusionRaw_NilExpressionPlan(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{}
	result := buildEmotionFusionRaw(runtime, "Amitia")
	if result != "" {
		t.Error("nil expression plan should return empty string")
	}
}

func TestBuildEmotionFusionRaw_ProducesFullSection(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{
		ExpressionPlan: &decision.ExpressionPlan{
			Tone:             decision.ExpressionToneWarm,
			EmotionIntensity: 0.8,
		},
	}
	result := buildEmotionFusionRaw(runtime, "Amitia")
	if result == "" {
		t.Fatal("should produce non-empty section")
	}
	requiredParts := []string{
		"行为优先级",
		"人格基底",
		"动态情绪",
		"甜蜜依恋",
		"融合执行策略",
		"绝对禁止清单",
	}
	for _, part := range requiredParts {
		if !strings.Contains(result, part) {
			t.Errorf("result should contain %q", part)
		}
	}
}

func TestBuildEmotionFusionInput_AllEmotionsHaveValidAffSecAroDom(t *testing.T) {
	tones := []decision.ExpressionTone{
		decision.ExpressionToneWarm,
		decision.ExpressionToneNeutral,
		decision.ExpressionTonePlayful,
		decision.ExpressionToneConcerned,
		decision.ExpressionToneFirm,
		decision.ExpressionToneSoft,
	}

	for _, tone := range tones {
		runtime := &interaction.RuntimeAssembly{
			ExpressionPlan: &decision.ExpressionPlan{
				Tone:             tone,
				EmotionIntensity: 0.5,
			},
		}
		input := buildEmotionFusionInput(runtime)
		if input == nil {
			t.Fatalf("tone %s should produce non-nil input", tone)
		}
		if input.PrimaryLabel == "" {
			t.Errorf("tone %s should have a primary label", tone)
		}
		if input.Aff < -1.0 || input.Aff > 1.0 {
			t.Errorf("tone %s aff out of range: %f", tone, input.Aff)
		}
		if input.Sec < -1.0 || input.Sec > 1.0 {
			t.Errorf("tone %s sec out of range: %f", tone, input.Sec)
		}
		if input.Aro < -1.0 || input.Aro > 1.0 {
			t.Errorf("tone %s aro out of range: %f", tone, input.Aro)
		}
		if input.Dom < -1.0 || input.Dom > 1.0 {
			t.Errorf("tone %s dom out of range: %f", tone, input.Dom)
		}
	}
}

func TestBuildEmotionFusionRaw_DifferentPersonalitiesProduceDifferentOutput(t *testing.T) {
	exprPlan := &decision.ExpressionPlan{
		Tone:             decision.ExpressionTonePlayful,
		EmotionIntensity: 0.6,
	}

	runtimeA := &interaction.RuntimeAssembly{
		ExpressionPlan: exprPlan,
		Context: interaction.ContextSnapshot{
			RuntimeProfile: interaction.FieldReady(interaction.RuntimeProfile{
				SpeakingStyle:     "傲娇冷淡，言不由衷",
				BoundaryRules:     "内心想靠近但嘴上不承认",
				PersonalityConfig: map[string]interface{}{"catchphrases": []interface{}{"哼", "切"}},
			}, "profile", "v1"),
		},
	}
	runtimeB := &interaction.RuntimeAssembly{
		ExpressionPlan: exprPlan,
		Context: interaction.ContextSnapshot{
			RuntimeProfile: interaction.FieldReady(interaction.RuntimeProfile{
				SpeakingStyle:     "温和体贴，真诚表达",
				BoundaryRules:     "愿意敞开心扉",
				PersonalityConfig: map[string]interface{}{"catchphrases": []interface{}{"嗯", "好呀"}},
			}, "profile", "v1"),
		},
	}

	resultA := buildEmotionFusionRaw(runtimeA, "傲娇A")
	resultB := buildEmotionFusionRaw(runtimeB, "温柔B")

	if resultA == "" || resultB == "" {
		t.Fatal("both should produce non-empty output")
	}
	if resultA == resultB {
		t.Error("different personalities should produce different output")
	}

	if !strings.Contains(resultA, "傲娇冷淡") {
		t.Error("resultA should contain tsundere speaking style")
	}
	if !strings.Contains(resultB, "温和体贴") {
		t.Error("resultB should contain gentle speaking style")
	}

	if !strings.Contains(resultA, "内心想靠近但嘴上不承认") {
		t.Error("resultA should contain tsundere core conflict")
	}
	if !strings.Contains(resultB, "愿意敞开心扉") {
		t.Error("resultB should contain gentle core conflict")
	}
}
