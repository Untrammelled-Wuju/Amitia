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
	PersonalityMatch float64
	RelationshipBias float64
	UserPreference   float64
	WeightedTotal    float64
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
	result.UserPreference = ComputeUserPreferenceScore(candidate, input.UserPreferences)
	result.WeightedTotal = round4(
		result.PersonalityMatch*config.PersonalityMatchWeight +
			result.RelationshipBias*config.RelationshipStateWeight +
			result.UserPreference*config.UserPreferenceWeight,
	)
	return result
}

// ApplyUserPreferenceSignals 只写入 UserPreferenceScore, 不修改 FinalScore
func ApplyUserPreferenceSignals(candidates []BehaviorCandidate, preferences map[string]float64) []BehaviorCandidate {
	result := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		next.UserPreferenceScore = ComputeUserPreferenceScore(next, preferences)
		result = append(result, next)
	}
	return result
}

// ApplySoftPreferencesToAll 保留兼容: 只写入 UserPreferenceScore, 不再修改 FinalScore
func ApplySoftPreferencesToAll(candidates []BehaviorCandidate, input SoftPreferenceInput, config SoftPreferenceConfig) []BehaviorCandidate {
	return ApplyUserPreferenceSignals(candidates, input.UserPreferences)
}

func computeRelationshipStateBias(candidate BehaviorCandidate, rel RelationshipSnapshot) float64 {
	if len(rel.Dimensions) == 0 {
		return 0
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
	return clampSignedScore(bias)
}

// ComputeUserPreferenceScore 返回候选对应的用户偏好分
// 优先 Candidate ID, 其次 BehaviorTag
// 无偏好返回 0, 不允许默认 0.5
// 支持 -1.0 ~ +1.0 表示负偏好
func ComputeUserPreferenceScore(candidate BehaviorCandidate, preferences map[string]float64) float64 {
	if preferences == nil {
		return 0
	}
	if val, ok := preferences[candidate.ID]; ok {
		return clampSignedScore(val)
	}
	if val, ok := preferences[string(candidate.Tag)]; ok {
		return clampSignedScore(val)
	}
	return 0
}

func clampSignedScore(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}
