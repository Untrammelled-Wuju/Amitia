package decision

import "testing"

func TestApplySoftPreferencesUsesPersonalityWeights(t *testing.T) {
	input := SoftPreferenceInput{
		PersonalityWeights: map[BehaviorTag]float64{
			BehaviorTagOfferSupport: 0.8,
		},
		Relationship: RelationshipSnapshot{},
		UserPreferences: nil,
	}
	config := DefaultSoftPreferenceConfig()
	candidate := BehaviorCandidate{ID: "offer_support", Tag: BehaviorTagOfferSupport}
	result := ApplySoftPreferences(candidate, input, config)
	if result.PersonalityMatch != 0.8 {
		t.Fatalf("人格权重应匹配 0.8, 实际 %f", result.PersonalityMatch)
	}
}

func TestApplySoftPreferencesDefaultsWhenNoMatch(t *testing.T) {
	input := SoftPreferenceInput{
		PersonalityWeights: map[BehaviorTag]float64{},
		Relationship:       RelationshipSnapshot{},
		UserPreferences:    nil,
	}
	config := DefaultSoftPreferenceConfig()
	candidate := BehaviorCandidate{ID: "chat_reply", Tag: BehaviorTagReply}
	result := ApplySoftPreferences(candidate, input, config)
	if result.PersonalityMatch != 0.3 {
		t.Fatalf("无匹配时人格权重应为 0.3, 实际 %f", result.PersonalityMatch)
	}
}

func TestComputeRelationshipStateBiasHighTrust(t *testing.T) {
	rel := RelationshipSnapshot{
		Dimensions: map[RelationshipDimension]RelationshipDimensionValue{
			RelationshipTrust:    {Value: 0.9},
			RelationshipIntimacy: {Value: 0.7},
			RelationshipConflict: {Value: 0.1},
		},
	}
	candidate := BehaviorCandidate{ID: "offer_support"}
	bias := computeRelationshipStateBias(candidate, rel)
	if bias < 0.7 {
		t.Fatalf("高信任关系应产生高偏差值, 实际 %f", bias)
	}
}

func TestComputeRelationshipStateBiasSetBoundaryHighConflict(t *testing.T) {
	rel := RelationshipSnapshot{
		Dimensions: map[RelationshipDimension]RelationshipDimensionValue{
			RelationshipTrust:    {Value: 0.4},
			RelationshipIntimacy: {Value: 0.3},
			RelationshipConflict: {Value: 0.8},
		},
	}
	candidate := BehaviorCandidate{ID: "set_boundary"}
	bias := computeRelationshipStateBias(candidate, rel)
	if bias <= 0.5 {
		t.Fatalf("高冲突时设立边界应有提升, 实际 %f", bias)
	}
}

func TestComputeUserPreferenceBias(t *testing.T) {
	prefs := map[string]float64{
		"chat_reply": 0.9,
	}
	candidate := BehaviorCandidate{ID: "chat_reply"}
	bias := computeUserPreferenceBias(candidate, prefs)
	if bias != 0.9 {
		t.Fatalf("用户偏好应匹配 0.9, 实际 %f", bias)
	}
}

func TestApplySoftPreferencesToAllModifiesScores(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", Tag: BehaviorTagReply, FinalScore: 0.5},
		{ID: "offer_support", Tag: BehaviorTagOfferSupport, FinalScore: 0.4},
	}
	input := SoftPreferenceInput{
		PersonalityWeights: map[BehaviorTag]float64{
			BehaviorTagOfferSupport: 0.9,
			BehaviorTagReply:        0.3,
		},
	}
	result := ApplySoftPreferencesToAll(candidates, input, DefaultSoftPreferenceConfig())
	if len(result) != 2 {
		t.Fatalf("应有 2 个候选, 实际 %d", len(result))
	}
}
