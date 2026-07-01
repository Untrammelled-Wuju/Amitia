package interaction

import "sort"

type ExpressionRenderConstraints struct {
	DisabledTones         []ExpressionTone `json:"disabledTones,omitempty"`
	DisabledEmotionKinds  []string         `json:"disabledEmotionKinds,omitempty"`
	DisabledPromptTypes   []string         `json:"disabledPromptTypes,omitempty"`
	DisabledGuardClaims   []string         `json:"disabledGuardClaims,omitempty"`
	DisabledGuardPatterns []string         `json:"disabledGuardPatterns,omitempty"`
	MinIntensity          float64          `json:"minIntensity"`
	MaxIntensity          float64          `json:"maxIntensity"`
}

type ExpressionRenderResult struct {
	Plan        ExpressionPlan `json:"plan"`
	Diagnostics []string       `json:"diagnostics,omitempty"`
}

func RenderExpressionStrategy(plan ExpressionPlan, constraints ExpressionRenderConstraints) ExpressionRenderResult {
	result := ExpressionRenderResult{Plan: plan}
	normalizeExpressionDefaults(&result.Plan, &result.Diagnostics)
	normalizeExpressionPolicy(&result.Plan.Policy, &result.Diagnostics)
	normalizeExpressionIntensityBounds(&constraints, &result.Diagnostics)
	result.Plan.Tones = normalizeExpressionTones(result.Plan.Tones, constraints.DisabledTones, &result.Diagnostics)
	result.Plan.EmotionPresentation = normalizeEmotionPresentation(result.Plan.EmotionPresentation, constraints, &result.Diagnostics)
	result.Plan.PromptSections = normalizePromptSections(result.Plan.PromptSections, constraints.DisabledPromptTypes, &result.Diagnostics)
	result.Plan.OutputGuards = normalizeOutputGuards(result.Plan.OutputGuards, constraints, &result.Diagnostics)
	result.Plan.Audit.Diagnostics = appendUniqueStrings(result.Plan.Audit.Diagnostics, result.Diagnostics...)
	return result
}

func normalizeExpressionDefaults(plan *ExpressionPlan, diagnostics *[]string) {
	if plan.Version == "" {
		plan.Version = ExpressionPlanVersionV1
		*diagnostics = append(*diagnostics, "default_version")
	}
	if plan.Policy.Version == "" {
		plan.Policy.Version = ExpressionPlanVersionV1
		*diagnostics = append(*diagnostics, "default_policy_version")
	}
	if len(plan.Tones) == 0 {
		plan.Tones = []ExpressionTone{ExpressionToneWarm, ExpressionToneRational}
		*diagnostics = append(*diagnostics, "default_tones")
	}
	if plan.Policy.Directness == "" {
		plan.Policy.Directness = ExpressionDirectnessBalanced
		*diagnostics = append(*diagnostics, "default_directness")
	}
	if plan.Policy.MaxCharacters <= 0 {
		plan.Policy.MaxCharacters = 240
		*diagnostics = append(*diagnostics, "default_max_characters")
	}
	if plan.Policy.MinCharacters < 0 {
		plan.Policy.MinCharacters = 0
		*diagnostics = append(*diagnostics, "clamp_min_characters")
	}
	if plan.Policy.MinCharacters > plan.Policy.MaxCharacters {
		plan.Policy.MinCharacters = plan.Policy.MaxCharacters
		*diagnostics = append(*diagnostics, "clamp_character_range")
	}
	if plan.Policy.MaxSentences <= 0 {
		plan.Policy.MaxSentences = 3
		*diagnostics = append(*diagnostics, "default_max_sentences")
	}
	if plan.Policy.MinSentences < 0 {
		plan.Policy.MinSentences = 0
		*diagnostics = append(*diagnostics, "clamp_min_sentences")
	}
	if plan.Policy.MinSentences > plan.Policy.MaxSentences {
		plan.Policy.MinSentences = plan.Policy.MaxSentences
		*diagnostics = append(*diagnostics, "clamp_sentence_range")
	}
}

func normalizeExpressionPolicy(policy *ExpressionPolicy, diagnostics *[]string) {
	policy.AdviceBias = clampExpressionValue(policy.AdviceBias, "clamp_advice_bias", diagnostics)
	policy.Warmth = clampExpressionValue(policy.Warmth, "clamp_warmth", diagnostics)
	policy.Rationality = clampExpressionValue(policy.Rationality, "clamp_rationality", diagnostics)
	policy.Playfulness = clampExpressionValue(policy.Playfulness, "clamp_playfulness", diagnostics)
	policy.Intimacy = clampExpressionValue(policy.Intimacy, "clamp_intimacy", diagnostics)
	policy.EmotionalDisclosure = clampExpressionValue(policy.EmotionalDisclosure, "clamp_emotional_disclosure", diagnostics)
}

func normalizeExpressionIntensityBounds(constraints *ExpressionRenderConstraints, diagnostics *[]string) {
	if constraints.MaxIntensity <= 0 {
		constraints.MaxIntensity = 1
		*diagnostics = append(*diagnostics, "default_max_intensity")
	}
	if constraints.MinIntensity < 0 {
		constraints.MinIntensity = 0
		*diagnostics = append(*diagnostics, "clamp_min_intensity")
	}
	if constraints.MaxIntensity > 1 {
		constraints.MaxIntensity = 1
		*diagnostics = append(*diagnostics, "clamp_max_intensity")
	}
	if constraints.MinIntensity > constraints.MaxIntensity {
		constraints.MinIntensity = constraints.MaxIntensity
		*diagnostics = append(*diagnostics, "clamp_intensity_range")
	}
}

func normalizeExpressionTones(tones []ExpressionTone, disabled []ExpressionTone, diagnostics *[]string) []ExpressionTone {
	disabledSet := expressionToneSet(disabled)
	seen := map[ExpressionTone]bool{}
	filtered := make([]ExpressionTone, 0, len(tones))
	for _, tone := range tones {
		if disabledSet[tone] {
			*diagnostics = append(*diagnostics, "filtered_tone:"+string(tone))
			continue
		}
		if seen[tone] {
			*diagnostics = append(*diagnostics, "dedup_tone:"+string(tone))
			continue
		}
		seen[tone] = true
		filtered = append(filtered, tone)
	}
	if len(filtered) == 0 {
		for _, tone := range []ExpressionTone{ExpressionToneWarm, ExpressionToneRational, ExpressionToneReserved} {
			if !disabledSet[tone] {
				filtered = append(filtered, tone)
				*diagnostics = append(*diagnostics, "fallback_tone:"+string(tone))
				break
			}
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return expressionToneRank(filtered[i]) < expressionToneRank(filtered[j])
	})
	return filtered
}

func normalizeEmotionPresentation(items []EmotionPresentation, constraints ExpressionRenderConstraints, diagnostics *[]string) []EmotionPresentation {
	disabled := stringSet(constraints.DisabledEmotionKinds)
	filtered := make([]EmotionPresentation, 0, len(items))
	for _, item := range items {
		if disabled[item.Kind] {
			*diagnostics = append(*diagnostics, "filtered_emotion:"+item.Kind)
			continue
		}
		next := item
		if next.Intensity < constraints.MinIntensity {
			next.Intensity = constraints.MinIntensity
			*diagnostics = append(*diagnostics, "clamp_emotion_min:"+item.Kind)
		}
		if next.Intensity > constraints.MaxIntensity {
			next.Intensity = constraints.MaxIntensity
			*diagnostics = append(*diagnostics, "clamp_emotion_max:"+item.Kind)
		}
		filtered = append(filtered, next)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Intensity > filtered[j].Intensity
	})
	return filtered
}

func normalizePromptSections(items []PromptSectionRef, disabled []string, diagnostics *[]string) []PromptSectionRef {
	disabledSet := stringSet(disabled)
	filtered := make([]PromptSectionRef, 0, len(items))
	for _, item := range items {
		if disabledSet[item.Type] {
			*diagnostics = append(*diagnostics, "filtered_prompt:"+item.Type)
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Priority > filtered[j].Priority
	})
	return filtered
}

func normalizeOutputGuards(guards OutputGuardSet, constraints ExpressionRenderConstraints, diagnostics *[]string) OutputGuardSet {
	guards.BlockedClaims = filterStrings(guards.BlockedClaims, stringSet(constraints.DisabledGuardClaims), "filtered_claim:", diagnostics)
	guards.InjectionPatterns = filterStrings(guards.InjectionPatterns, stringSet(constraints.DisabledGuardPatterns), "filtered_pattern:", diagnostics)
	return guards
}

func clampExpressionValue(value float64, diagnostic string, diagnostics *[]string) float64 {
	if value < 0 {
		*diagnostics = append(*diagnostics, diagnostic)
		return 0
	}
	if value > 1 {
		*diagnostics = append(*diagnostics, diagnostic)
		return 1
	}
	return value
}

func expressionToneSet(values []ExpressionTone) map[ExpressionTone]bool {
	result := make(map[ExpressionTone]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func filterStrings(values []string, disabled map[string]bool, diagnosticPrefix string, diagnostics *[]string) []string {
	if len(values) == 0 || len(disabled) == 0 {
		return values
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if disabled[value] {
			*diagnostics = append(*diagnostics, diagnosticPrefix+value)
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func expressionToneRank(tone ExpressionTone) int {
	switch tone {
	case ExpressionToneWarm:
		return 10
	case ExpressionToneRational:
		return 20
	case ExpressionTonePlayful:
		return 30
	case ExpressionToneIntimate:
		return 40
	case ExpressionToneReserved:
		return 50
	case ExpressionToneRepairing:
		return 60
	default:
		return 100
	}
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := stringSet(values)
	for _, addition := range additions {
		if seen[addition] {
			continue
		}
		values = append(values, addition)
		seen[addition] = true
	}
	return values
}
