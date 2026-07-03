package expression

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestMapExpressionToVoice_UnsupportedChannelReturnsNeutral(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-1",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "joy", Intensity: 0.8},
		},
	}
	vp := MapExpressionToVoiceSafe(plan, false)
	if vp.EmotionTier != VoiceEmotionNeutral {
		t.Fatalf("expected neutral for unsupported channel, got %s", vp.EmotionTier)
	}
	if vp.Trace.FallbackReason != "channel_unsupported" {
		t.Fatalf("expected fallback reason channel_unsupported, got %s", vp.Trace.FallbackReason)
	}
}

func TestMapExpressionToVoice_JoyMapsToPositive(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-2",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "joy", Intensity: 0.7},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionPositive {
		t.Fatalf("expected positive, got %s", vp.EmotionTier)
	}
	if vp.Speed <= 1.0 {
		t.Fatalf("expected speed > 1.0 for positive, got %f", vp.Speed)
	}
}

func TestMapExpressionToVoice_SadnessMapsToNegative(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-3",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "sadness", Intensity: 0.5},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionNegative {
		t.Fatalf("expected negative, got %s", vp.EmotionTier)
	}
	if vp.Speed >= 1.0 {
		t.Fatalf("expected speed < 1.0 for negative, got %f", vp.Speed)
	}
}

func TestMapExpressionToVoice_NegativeEmotionSafetyCap(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-4",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "anger", Intensity: 0.95},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.Intensity > negativeEmotionSafetyCap {
		t.Fatalf("expected intensity capped at %f, got %f", negativeEmotionSafetyCap, vp.Intensity)
	}
	if !vp.Trace.SafetyClamped {
		t.Fatal("expected safetyClamped to be true")
	}
}

func TestMapExpressionToVoice_CareMapsToCaring(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-5",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "care", Intensity: 0.6},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionCaring {
		t.Fatalf("expected caring, got %s", vp.EmotionTier)
	}
}

func TestMapExpressionToVoice_HumorMapsToHumorous(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-6",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "humor", Intensity: 0.9},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionHumorous {
		t.Fatalf("expected humorous, got %s", vp.EmotionTier)
	}
}

func TestMapExpressionToVoice_FallbackToTonesWhenNoEmotion(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-7",
		Tones: []interaction.ExpressionTone{
			interaction.ExpressionToneWarm,
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionCaring {
		t.Fatalf("expected caring from warm tone, got %s", vp.EmotionTier)
	}
}

func TestMapExpressionToVoice_EmptyPlanReturnsNeutral(t *testing.T) {
	plan := interaction.ExpressionPlan{ID: "plan-8"}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionNeutral {
		t.Fatalf("expected neutral for empty plan, got %s", vp.EmotionTier)
	}
}

func TestMapExpressionToVoice_TraceContainsMappedData(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-9",
		Tones: []interaction.ExpressionTone{
			interaction.ExpressionToneWarm,
			interaction.ExpressionTonePlayful,
		},
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "joy", Intensity: 0.7},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.Trace.SourcePlanID != "plan-9" {
		t.Fatalf("expected trace sourcePlanId plan-9, got %s", vp.Trace.SourcePlanID)
	}
	if len(vp.Trace.MappedTones) != 2 {
		t.Fatalf("expected 2 mapped tones, got %d", len(vp.Trace.MappedTones))
	}
	if len(vp.Trace.MappedEmotions) != 1 {
		t.Fatalf("expected 1 mapped emotion, got %d", len(vp.Trace.MappedEmotions))
	}
	if vp.Trace.GeneratedAt.IsZero() {
		t.Fatal("expected non-zero generatedAt in trace")
	}
}

func TestMapExpressionToVoice_SpeedAndPauseBounds(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID: "plan-10",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "humor", Intensity: 1.0},
		},
	}
	vp := MapExpressionToVoice(plan)
	if vp.Speed > 1.3 {
		t.Fatalf("expected speed capped at 1.3, got %f", vp.Speed)
	}
	plan2 := interaction.ExpressionPlan{
		ID: "plan-11",
		EmotionPresentation: []interaction.EmotionPresentation{
			{Kind: "sadness", Intensity: 0.6},
		},
	}
	vp2 := MapExpressionToVoice(plan2)
	if vp2.Speed < 0.7 {
		t.Fatalf("expected speed min 0.7, got %f", vp2.Speed)
	}
}

func TestMapExpressionToVoice_OnlyFiveTiersExposed(t *testing.T) {
	allEmotions := []string{
		"joy", "sadness", "anger", "fear", "care", "love",
		"humor", "playful", "surprise", "disgust", "frustration",
		"gratitude", "hope", "anxiety", "warmth", "empathy",
	}
	for _, emo := range allEmotions {
		plan := interaction.ExpressionPlan{
			ID: "plan-tier-" + emo,
			EmotionPresentation: []interaction.EmotionPresentation{
				{Kind: emo, Intensity: 0.5},
			},
		}
		vp := MapExpressionToVoice(plan)
		switch vp.EmotionTier {
		case VoiceEmotionPositive, VoiceEmotionNeutral, VoiceEmotionNegative,
			VoiceEmotionCaring, VoiceEmotionHumorous:
		default:
			t.Fatalf("emotion %s mapped to invalid tier %s", emo, vp.EmotionTier)
		}
	}
}

func TestMapExpressionToVoice_RepairingToneMapsToCaring(t *testing.T) {
	plan := interaction.ExpressionPlan{
		ID:    "plan-repair",
		Tones: []interaction.ExpressionTone{interaction.ExpressionToneRepairing},
	}
	vp := MapExpressionToVoice(plan)
	if vp.EmotionTier != VoiceEmotionCaring {
		t.Fatalf("expected caring from repairing tone, got %s", vp.EmotionTier)
	}
}

func TestBuildAudioRequest_ContainsAllParams(t *testing.T) {
	vp := VoiceParams{
		Speed:       1.05,
		Pause:       0.4,
		EmotionTier: VoiceEmotionPositive,
		Intensity:   0.7,
	}
	req := BuildAudioRequest(vp, "hello world")
	if req["text"] != "hello world" {
		t.Fatalf("expected text hello world, got %v", req["text"])
	}
	if req["speed"] != 1.05 {
		t.Fatalf("expected speed 1.05, got %v", req["speed"])
	}
}

func TestBuildAudioRequestWithTrace(t *testing.T) {
	vp := VoiceParams{
		Speed:       1.0,
		Pause:       0.5,
		EmotionTier: VoiceEmotionNeutral,
		Intensity:   0.0,
		Trace: VoiceTrace{
			SourcePlanID: "plan-trace",
			MappedTones:  []string{"warm"},
		},
	}
	req, traceBytes := BuildAudioRequestWithTrace(vp, "test")
	if req == nil {
		t.Fatal("expected non-nil audio request")
	}
	if traceBytes == nil {
		t.Fatal("expected non-nil trace bytes")
	}
	var decodedTrace VoiceTrace
	if err := json.Unmarshal(traceBytes, &decodedTrace); err != nil {
		t.Fatalf("expected valid trace JSON, got error: %v", err)
	}
}
