package decision

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBehaviorPlanJSONKeepsPersonalityAndPsycheInputs(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	plan := BehaviorPlan{
		Version:     PlanVersionV1,
		ID:          "plan-1",
		UserID:      "user-1",
		CharacterID: "char-1",
		CreatedAt:   now,
		Selected: BehaviorCandidate{
			ID:               "candidate-1",
			Tag:              BehaviorTagOfferSupport,
			Channel:          BehaviorChannelChat,
			BaseScore:        0.45,
			PersonalityScore: 0.2,
			NeedScore:        0.1,
			RiskScore:        0.05,
			FinalScore:       0.7,
			Reasons: []BehaviorReason{
				{Source: "personality", Key: "warmth", Delta: 0.2},
			},
		},
		Priority:        BehaviorPriorityNormal,
		SafetyLevel:     BehaviorSafetyLevelConservative,
		NeedsExpression: true,
		Personality: CompiledPersonalityRef{
			Version:           "runtime-profile-v1",
			SourceCharacterID: "char-1",
			RawConfig: map[string]any{
				"proactivity": 0.8,
			},
			BehaviorWeights: map[BehaviorTag]float64{
				BehaviorTagOfferSupport: 0.2,
			},
			ExpressionPolicyKey: "warm-short",
		},
		Psyche: PsycheSignalSet{
			Emotions: []EmotionSignal{
				{Kind: "care", Intensity: 0.75, SourceEvent: "evt-1", OnsetAt: now, DecayProfile: "medium"},
			},
			Mood:          ScalarSignal{Value: 0.3, Baseline: 0, Confidence: 0.9},
			Stress:        ScalarSignal{Value: 0.2, Baseline: 0.1, Confidence: 0.8},
			CognitiveLoad: ScalarSignal{Value: 0.4, Baseline: 0.2, Confidence: 0.8},
			Needs: []NeedSignal{
				{Kind: "connection", Level: 0.65, Baseline: 0.4, Trend: 0.1},
			},
			Regulation: RegulationSignal{Strategy: "reappraise", ExpressionMode: "soften", AppraisalID: "appraisal-1", RevisionID: "revision-1"},
		},
		Relationship: RelationshipSnapshot{
			UserID:      "user-1",
			CharacterID: "char-1",
			Dimensions: map[RelationshipDimension]RelationshipDimensionValue{
				RelationshipTrust: {Value: 0.6, Baseline: 0.5, EvidenceIDs: []string{"evidence-1"}, LastChangedAt: now},
			},
			LastChangedAt: now,
		},
		Life:  LifeSnapshot{Energy: 0.7, Fatigue: 0.2, Busy: 0.1, Activity: "available", Availability: "open", Source: "neutral-default"},
		Audit: BehaviorAudit{FormulaVersion: "behavior-formula-v1", ParameterVersion: "params-v1", SnapshotID: "snapshot-1"},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var decoded BehaviorPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != PlanVersionV1 {
		t.Fatalf("unexpected version: %s", decoded.Version)
	}
	if decoded.Personality.RawConfig["proactivity"].(float64) != 0.8 {
		t.Fatalf("personality config not preserved: %#v", decoded.Personality.RawConfig)
	}
	if decoded.Psyche.Emotions[0].Kind != "care" || decoded.Psyche.Needs[0].Kind != "connection" {
		t.Fatalf("psyche signals not preserved: %#v", decoded.Psyche)
	}
	if decoded.Relationship.Dimensions[RelationshipTrust].EvidenceIDs[0] != "evidence-1" {
		t.Fatalf("relationship evidence not preserved: %#v", decoded.Relationship)
	}
	if !decoded.NeedsExpression {
		t.Fatal("expected expression handoff flag")
	}
}
