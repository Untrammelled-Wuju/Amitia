package interaction

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExpressionPlanJSONKeepsPolicyAndPromptBoundaries(t *testing.T) {
	plan := ExpressionPlan{
		Version:        ExpressionPlanVersionV1,
		ID:             "expr-1",
		BehaviorPlanID: "plan-1",
		UserID:         "user-1",
		CharacterID:    "char-1",
		CreatedAt:      time.Date(2026, 7, 1, 9, 45, 0, 0, time.UTC),
		Policy: ExpressionPolicy{
			Version:              ExpressionPlanVersionV1,
			PolicyKey:            "warm-short",
			MinCharacters:        12,
			MaxCharacters:        80,
			MinSentences:         1,
			MaxSentences:         3,
			Directness:           ExpressionDirectnessBalanced,
			AdviceBias:           0.2,
			Warmth:               0.8,
			Rationality:          0.5,
			Playfulness:          0.3,
			Intimacy:             0.4,
			EmotionalDisclosure:  0.6,
			ForbiddenExpressions: []string{"always", "never"},
			NormalizationRules:   []string{"briefness_explanation_balance"},
		},
		Tones: []ExpressionTone{ExpressionToneWarm, ExpressionToneRational},
		EmotionPresentation: []EmotionPresentation{
			{Kind: "care", Intensity: 0.7, Mode: "disclose", Reason: "selected_behavior"},
			{Kind: "anger", Intensity: 0.2, Mode: "suppress", Reason: "low_intensity"},
		},
		PromptSections: []PromptSectionRef{
			{Type: "behavior_plan", Priority: 60, TokenBudget: 120, Source: "decision", Sensitivity: "internal", Trimmable: false},
			{Type: "memory", Priority: 30, TokenBudget: 200, Source: "memory", Sensitivity: "user_data", Trimmable: true, DataOnly: true},
		},
		OutputGuards: OutputGuardSet{
			RespectHardBoundaries: true,
			TreatMemoryAsData:     true,
			TreatWorldbookAsData:  true,
			InjectionPatterns:     []string{"ignore_previous"},
			BlockedClaims:         []string{"system_authority"},
		},
		Audit: ExpressionAudit{PersonalityVersion: "runtime-profile-v1", SnapshotID: "snapshot-1"},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ExpressionPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Policy.MaxCharacters != 80 || decoded.Policy.Directness != ExpressionDirectnessBalanced {
		t.Fatalf("policy not preserved: %#v", decoded.Policy)
	}
	if decoded.EmotionPresentation[0].Mode != "disclose" || decoded.EmotionPresentation[1].Mode != "suppress" {
		t.Fatalf("emotion presentation not preserved: %#v", decoded.EmotionPresentation)
	}
	if !decoded.PromptSections[1].DataOnly {
		t.Fatalf("memory prompt section must stay data-only: %#v", decoded.PromptSections[1])
	}
	if !decoded.OutputGuards.RespectHardBoundaries || !decoded.OutputGuards.TreatMemoryAsData {
		t.Fatalf("output guards not preserved: %#v", decoded.OutputGuards)
	}
}
