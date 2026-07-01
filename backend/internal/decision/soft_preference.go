package decision

type SoftPreferenceConfig struct {
	PersonalityMatchWeight  float64
	RelationshipStateWeight float64
	UserPreferenceWeight    float64
}

func DefaultSoftPreferenceConfig() SoftPreferenceConfig {
	return SoftPreferenceConfig{
		PersonalityMatchWeight:  0.35,
		RelationshipStateWeight: 0.35,
		UserPreferenceWeight:    0.30,
	}
}

type SoftPreferenceInput struct {
	PersonalityWeights map[BehaviorTag]float64
	Relationship       RelationshipSnapshot
	UserPreferences    map[string]float64
}

type PreferencesResult struct {
	PersonalityMatch  float64
	RelationshipBias  float64
	UserPreference    float64
	WeightedTotal     float64
}

func ApplySoftPreferences(candidate BehaviorCandidate, input SoftPreferenceInput, config SoftPreferenceConfig) PreferencesResult {
	result := PreferencesResult{}
	if config.PersonalityMatchWeight <= 0 {
		config.PersonalityMatchWeight = 0.35
	}
	if config.RelationshipStateWeight <= 0 {
		config.RelationshipStateWeight = 0.35
	}
	if config.UserPreferenceWeight <= 0 {
		config.UserPreferenceWeight = 0.30
	}
	if weight, ok := input.PersonalityWeights[candidate.Tag]; ok {
		result.PersonalityMatch = weight
	} else {
		result.PersonalityMatch = 0.3
	}
	result.RelationshipBias = computeRelationshipStateBias(candidate, input.Relationship)
	result.UserPreference = computeUserPreferenceBias(candidate, input.UserPreferences)
	result.WeightedTotal = round4(
		result.PersonalityMatch*config.PersonalityMatchWeight +
			result.RelationshipBias*config.RelationshipStateWeight +
			result.UserPreference*config.UserPreferenceWeight,
	)
	return result
}

func ApplySoftPreferencesToAll(candidates []BehaviorCandidate, input SoftPreferenceInput, config SoftPreferenceConfig) []BehaviorCandidate {
	result := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		prefs := ApplySoftPreferences(next, input, config)
		next.FinalScore = round4(next.FinalScore + prefs.WeightedTotal*0.1)
		result = append(result, next)
	}
	return result
}

func computeRelationshipStateBias(candidate BehaviorCandidate, rel RelationshipSnapshot) float64 {
	if len(rel.Dimensions) == 0 {
		return 0.5
	}
	trustVal := 0.5
	if v, ok := rel.Dimensions[RelationshipTrust]; ok {
		trustVal = v.Value
	}
	intimacyVal := 0.5
	if v, ok := rel.Dimensions[RelationshipIntimacy]; ok {
		intimacyVal = v.Value
	}
	conflictVal := 0.0
	if v, ok := rel.Dimensions[RelationshipConflict]; ok {
		conflictVal = v.Value
	}
	bias := trustVal*0.4 + intimacyVal*0.3 + (1.0-conflictVal)*0.3
	if candidate.ID == "set_boundary" && conflictVal > 0.5 {
		bias += 0.2
	}
	if candidate.ID == "offer_support" && trustVal > 0.6 {
		bias += 0.15
	}
	return clamp01Val(bias)
}

func computeUserPreferenceBias(candidate BehaviorCandidate, prefs map[string]float64) float64 {
	if prefs == nil {
		return 0.5
	}
	if val, ok := prefs[candidate.ID]; ok {
		return clamp01Val(val)
	}
	if val, ok := prefs[string(candidate.Tag)]; ok {
		return clamp01Val(val)
	}
	return 0.5
}
