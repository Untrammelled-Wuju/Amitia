package interaction

import "testing"

func TestRenderExpressionStrategyFiltersDisabledEntries(t *testing.T) {
	plan := ExpressionPlan{
		Tones: []ExpressionTone{ExpressionTonePlayful, ExpressionToneWarm, ExpressionToneWarm},
		EmotionPresentation: []EmotionPresentation{
			{Kind: "joy", Intensity: 0.9, Mode: "show"},
			{Kind: "anger", Intensity: 0.4, Mode: "suppress"},
		},
		PromptSections: []PromptSectionRef{
			{Type: "memory", Priority: 20},
			{Type: "behavior_plan", Priority: 80},
		},
		OutputGuards: OutputGuardSet{
			BlockedClaims:     []string{"system_authority", "identity_override"},
			InjectionPatterns: []string{"ignore_previous", "roleplay_admin"},
		},
	}

	result := RenderExpressionStrategy(plan, ExpressionRenderConstraints{
		DisabledTones:         []ExpressionTone{ExpressionTonePlayful},
		DisabledEmotionKinds:  []string{"anger"},
		DisabledPromptTypes:   []string{"memory"},
		DisabledGuardClaims:   []string{"identity_override"},
		DisabledGuardPatterns: []string{"roleplay_admin"},
	})

	if len(result.Plan.Tones) != 1 || result.Plan.Tones[0] != ExpressionToneWarm {
		t.Fatalf("unexpected tones: %#v", result.Plan.Tones)
	}
	if len(result.Plan.EmotionPresentation) != 1 || result.Plan.EmotionPresentation[0].Kind != "joy" {
		t.Fatalf("unexpected emotions: %#v", result.Plan.EmotionPresentation)
	}
	if len(result.Plan.PromptSections) != 1 || result.Plan.PromptSections[0].Type != "behavior_plan" {
		t.Fatalf("unexpected prompt sections: %#v", result.Plan.PromptSections)
	}
	if len(result.Plan.OutputGuards.BlockedClaims) != 1 || result.Plan.OutputGuards.BlockedClaims[0] != "system_authority" {
		t.Fatalf("unexpected blocked claims: %#v", result.Plan.OutputGuards.BlockedClaims)
	}
	if len(result.Plan.OutputGuards.InjectionPatterns) != 1 || result.Plan.OutputGuards.InjectionPatterns[0] != "ignore_previous" {
		t.Fatalf("unexpected injection patterns: %#v", result.Plan.OutputGuards.InjectionPatterns)
	}
	if !containsExpressionDiagnostic(result.Diagnostics, "filtered_tone:playful") ||
		!containsExpressionDiagnostic(result.Diagnostics, "filtered_emotion:anger") ||
		!containsExpressionDiagnostic(result.Diagnostics, "filtered_prompt:memory") {
		t.Fatalf("missing diagnostics: %#v", result.Diagnostics)
	}
}

func TestRenderExpressionStrategyClampsIntensityAndPolicy(t *testing.T) {
	plan := ExpressionPlan{
		Policy: ExpressionPolicy{
			AdviceBias:          1.4,
			Warmth:              -0.2,
			EmotionalDisclosure: 1.3,
		},
		EmotionPresentation: []EmotionPresentation{
			{Kind: "care", Intensity: 1.4},
			{Kind: "fear", Intensity: -0.3},
		},
	}

	result := RenderExpressionStrategy(plan, ExpressionRenderConstraints{
		MinIntensity: -1,
		MaxIntensity: 2,
	})

	if result.Plan.Policy.AdviceBias != 1 || result.Plan.Policy.Warmth != 0 || result.Plan.Policy.EmotionalDisclosure != 1 {
		t.Fatalf("unexpected policy clamp: %#v", result.Plan.Policy)
	}
	if result.Plan.EmotionPresentation[0].Intensity != 1 || result.Plan.EmotionPresentation[1].Intensity != 0 {
		t.Fatalf("unexpected emotion clamp: %#v", result.Plan.EmotionPresentation)
	}
	if !containsExpressionDiagnostic(result.Diagnostics, "clamp_advice_bias") ||
		!containsExpressionDiagnostic(result.Diagnostics, "clamp_warmth") ||
		!containsExpressionDiagnostic(result.Diagnostics, "clamp_emotional_disclosure") ||
		!containsExpressionDiagnostic(result.Diagnostics, "clamp_max_intensity") ||
		!containsExpressionDiagnostic(result.Diagnostics, "clamp_min_intensity") {
		t.Fatalf("missing clamp diagnostics: %#v", result.Diagnostics)
	}
}

func TestRenderExpressionStrategyStableSorting(t *testing.T) {
	plan := ExpressionPlan{
		Tones: []ExpressionTone{ExpressionToneReserved, ExpressionToneWarm, ExpressionToneRational},
		EmotionPresentation: []EmotionPresentation{
			{Kind: "first", Intensity: 0.8},
			{Kind: "second", Intensity: 0.8},
			{Kind: "third", Intensity: 0.5},
		},
		PromptSections: []PromptSectionRef{
			{Type: "low", Priority: 10},
			{Type: "first", Priority: 50},
			{Type: "second", Priority: 50},
		},
	}

	result := RenderExpressionStrategy(plan, ExpressionRenderConstraints{})

	if result.Plan.Tones[0] != ExpressionToneWarm || result.Plan.Tones[1] != ExpressionToneRational || result.Plan.Tones[2] != ExpressionToneReserved {
		t.Fatalf("unexpected tone order: %#v", result.Plan.Tones)
	}
	if result.Plan.EmotionPresentation[0].Kind != "first" || result.Plan.EmotionPresentation[1].Kind != "second" {
		t.Fatalf("emotion order should stay stable on tie: %#v", result.Plan.EmotionPresentation)
	}
	if result.Plan.PromptSections[0].Type != "first" || result.Plan.PromptSections[1].Type != "second" {
		t.Fatalf("prompt section order should stay stable on tie: %#v", result.Plan.PromptSections)
	}
}

func TestRenderExpressionStrategyDefaultsOnEmptyInput(t *testing.T) {
	result := RenderExpressionStrategy(ExpressionPlan{}, ExpressionRenderConstraints{})

	if result.Plan.Version != ExpressionPlanVersionV1 || result.Plan.Policy.Version != ExpressionPlanVersionV1 {
		t.Fatalf("unexpected versions: %#v %#v", result.Plan.Version, result.Plan.Policy.Version)
	}
	if len(result.Plan.Tones) == 0 || result.Plan.Tones[0] != ExpressionToneWarm {
		t.Fatalf("unexpected default tones: %#v", result.Plan.Tones)
	}
	if result.Plan.Policy.Directness != ExpressionDirectnessBalanced {
		t.Fatalf("unexpected default directness: %#v", result.Plan.Policy.Directness)
	}
	if result.Plan.Policy.MaxCharacters != 240 || result.Plan.Policy.MaxSentences != 3 {
		t.Fatalf("unexpected default policy: %#v", result.Plan.Policy)
	}
	if !containsExpressionDiagnostic(result.Diagnostics, "default_tones") || !containsExpressionDiagnostic(result.Plan.Audit.Diagnostics, "default_policy_version") {
		t.Fatalf("missing default diagnostics: %#v %#v", result.Diagnostics, result.Plan.Audit.Diagnostics)
	}
}

func TestRenderExpressionStrategyProducesAuditDiagnostics(t *testing.T) {
	plan := ExpressionPlan{
		Audit: ExpressionAudit{Diagnostics: []string{"existing"}},
		Tones: []ExpressionTone{ExpressionToneRepairing},
	}

	result := RenderExpressionStrategy(plan, ExpressionRenderConstraints{
		DisabledTones: []ExpressionTone{ExpressionToneRepairing},
	})

	if len(result.Plan.Audit.Diagnostics) < 2 {
		t.Fatalf("expected merged diagnostics: %#v", result.Plan.Audit.Diagnostics)
	}
	if !containsExpressionDiagnostic(result.Plan.Audit.Diagnostics, "existing") ||
		!containsExpressionDiagnostic(result.Plan.Audit.Diagnostics, "filtered_tone:repairing") ||
		!containsExpressionDiagnostic(result.Plan.Audit.Diagnostics, "fallback_tone:warm") {
		t.Fatalf("unexpected audit diagnostics: %#v", result.Plan.Audit.Diagnostics)
	}
}

func containsExpressionDiagnostic(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
