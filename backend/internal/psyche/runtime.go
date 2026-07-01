package psyche

import "math"

const DefaultRuntimeVersion = "psyche-runtime-v1"

func ModulateRuntime(profile CompiledProfile, input RuntimeStateInput) RuntimeModulation {
	diagnostics := []string{}
	if profile.CompilerVersion == "" {
		profile = CompilePersonality(DefaultConfig())
		diagnostics = append(diagnostics, "profile_defaulted")
	}

	sources := map[string]string{}
	state := normalizeRuntimeState(input, sources, &diagnostics)
	influence := computeRuntimeInfluence(profile, state)
	internal := modulateInternal(profile.Internal, influence)
	appraisal := modulateAppraisal(profile.Appraisal, influence)
	recovery := modulateRecovery(profile.Recovery, influence)
	behavior := modulateBehavior(profile.Behavior, influence)
	expression := modulateExpression(profile.Expression, influence)

	return RuntimeModulation{
		Version:     DefaultRuntimeVersion,
		State:       state,
		Influence:   influence,
		Internal:    internal,
		Appraisal:   appraisal,
		Recovery:    recovery,
		Behavior:    behavior,
		Expression:  expression,
		Sources:     sources,
		Diagnostics: diagnostics,
	}
}

func normalizeRuntimeState(input RuntimeStateInput, sources map[string]string, diagnostics *[]string) RuntimeState {
	return RuntimeState{
		Stress:        resolveRuntimeScore("stress", input.Stress, 0, sources, diagnostics),
		Fatigue:       resolveRuntimeScore("fatigue", input.Fatigue, 0, sources, diagnostics),
		Arousal:       resolveRuntimeScore("arousal", input.Arousal, 35, sources, diagnostics),
		MoodPressure:  resolveRuntimeScore("moodPressure", input.MoodPressure, 0, sources, diagnostics),
		SocialLoad:    resolveRuntimeScore("socialLoad", input.SocialLoad, 35, sources, diagnostics),
		RecoveryHours: resolveRuntimeHours("recoveryHours", input.RecoveryHours, 0, 72, sources, diagnostics),
	}
}

func resolveRuntimeScore(name string, value *float64, fallback float64, sources map[string]string, diagnostics *[]string) float64 {
	if value == nil {
		sources[name] = "default"
		return round4(fallback / 100)
	}
	clamped := clampRange(0, 100, *value)
	if clamped != *value {
		sources[name] = "user_clamped"
		*diagnostics = append(*diagnostics, name+"_clamped")
		return round4(clamped / 100)
	}
	sources[name] = "user"
	return round4(clamped / 100)
}

func resolveRuntimeHours(name string, value *float64, fallback, maximum float64, sources map[string]string, diagnostics *[]string) float64 {
	if value == nil {
		sources[name] = "default"
		return fallback
	}
	clamped := clampRange(0, maximum, *value)
	if clamped != *value {
		sources[name] = "user_clamped"
		*diagnostics = append(*diagnostics, name+"_clamped")
		return round4(clamped)
	}
	sources[name] = "user"
	return round4(clamped)
}

func computeRuntimeInfluence(profile CompiledProfile, state RuntimeState) RuntimeInfluence {
	resilience := clamp01(profile.Recovery.ResilienceBias)
	recoveryHalfLife := maxFloat(1, profile.Recovery.StressHalfLifeHours)
	recoveryProgress := clamp01(1 - math.Pow(0.5, state.RecoveryHours/recoveryHalfLife))
	recoveryImpact := clamp01(recoveryProgress * profile.Recovery.MaxRecoveryRate)
	stressImpact := clamp01(state.Stress*(0.72+profile.Internal.StableCore.RejectionSensitivity*0.25) + state.MoodPressure*0.22 + state.SocialLoad*0.08 - recoveryImpact*0.45)
	fatigueImpact := clamp01(state.Fatigue*0.78 + state.Arousal*0.12 - recoveryImpact*0.3)
	regulation := clamp01(resilience + profile.Internal.StableCore.EmotionStability*0.28 + profile.Internal.StableCore.UncertaintyTolerance*0.16 - stressImpact*0.32 - fatigueImpact*0.18)
	expressionPenalty := clamp01(fatigueImpact*0.5 + stressImpact*0.25 + state.MoodPressure*0.15)
	pressure := clamp01(stressImpact*0.58 + fatigueImpact*0.24 + state.Arousal*0.1 + state.SocialLoad*0.08 - regulation*0.22)
	volatility := clamp01(pressure*0.55 + profile.Internal.StableCore.RejectionSensitivity*0.25 - regulation*0.18)

	return RuntimeInfluence{
		StressImpact:      round4(stressImpact),
		FatigueImpact:     round4(fatigueImpact),
		RecoveryImpact:    round4(recoveryImpact),
		Regulation:        round4(regulation),
		ExpressionPenalty: round4(expressionPenalty),
		Pressure:          round4(pressure),
		Volatility:        round4(volatility),
	}
}

func modulateInternal(base InternalModel, influence RuntimeInfluence) InternalModel {
	stress := influence.StressImpact
	fatigue := influence.FatigueImpact
	recovery := influence.RecoveryImpact
	regulation := influence.Regulation

	return InternalModel{
		StableCore: StableCoreLayer{
			SocialInitiative:     round4(clamp01(base.StableCore.SocialInitiative - stress*0.1 - fatigue*0.16 + recovery*0.08)),
			RejectionSensitivity: round4(clamp01(base.StableCore.RejectionSensitivity + stress*0.18 - regulation*0.06)),
			UncertaintyTolerance: round4(clamp01(base.StableCore.UncertaintyTolerance - stress*0.14 - fatigue*0.05 + recovery*0.08)),
			EmotionStability:     round4(clamp01(base.StableCore.EmotionStability - stress*0.16 - fatigue*0.08 + recovery*0.1)),
			BoundaryStrength:     round4(clamp01(base.StableCore.BoundaryStrength + stress*0.08 - recovery*0.04)),
		},
		Growth: GrowthLayer{
			Warmth:      round4(clamp01(base.Growth.Warmth - stress*0.08 - fatigue*0.06 + recovery*0.04)),
			Humor:       round4(clamp01(base.Growth.Humor - fatigue*0.18 - stress*0.05 + recovery*0.06)),
			Affection:   round4(clamp01(base.Growth.Affection - stress*0.07 - fatigue*0.05 + recovery*0.04)),
			SupportBias: round4(clamp01(base.Growth.SupportBias - stress*0.06 - fatigue*0.08 + recovery*0.05)),
		},
		Situational: SituationalLayer{
			Directness:        round4(clamp01(base.Situational.Directness - fatigue*0.08 + stress*0.05)),
			Verbosity:         round4(clamp01(base.Situational.Verbosity - fatigue*0.22 - stress*0.08 + recovery*0.08)),
			ConflictAvoidance: round4(clamp01(base.Situational.ConflictAvoidance + stress*0.14 + fatigue*0.06 - recovery*0.05)),
		},
	}
}

func modulateAppraisal(base AppraisalCoefficients, influence RuntimeInfluence) AppraisalCoefficients {
	stress := influence.StressImpact
	volatility := influence.Volatility
	recovery := influence.RecoveryImpact
	return AppraisalCoefficients{
		Version:               DefaultRuntimeVersion,
		Rejection:             round4(clampRange(0.2, 0.98, base.Rejection+stress*0.12-recovery*0.04)),
		RelationshipRelevance: round4(clampRange(0.2, 0.98, base.RelationshipRelevance+stress*0.04)),
		ExpectationGap:        round4(clampRange(0.2, 0.98, base.ExpectationGap+volatility*0.08-recovery*0.03)),
		Uncertainty:           round4(clampRange(0.2, 0.98, base.Uncertainty+stress*0.1-recovery*0.05)),
		Boundary:              round4(clampRange(0.2, 0.98, base.Boundary+stress*0.07)),
		AmplificationCap:      round4(clampRange(1.15, 2.4, base.AmplificationCap+stress*0.18+volatility*0.12-recovery*0.08)),
		Explanation:           base.Explanation,
	}
}

func modulateRecovery(base RecoveryProfile, influence RuntimeInfluence) RecoveryProfile {
	stress := influence.StressImpact
	fatigue := influence.FatigueImpact
	recovery := influence.RecoveryImpact
	minRecovery := clampRange(0.02, 0.22, base.MinRecoveryRate-stress*0.02+recovery*0.03)
	maxRecovery := clampRange(0.12, 0.75, base.MaxRecoveryRate-stress*0.05-fatigue*0.03+recovery*0.08)
	if maxRecovery < minRecovery {
		maxRecovery = minRecovery
	}
	return RecoveryProfile{
		Version:              DefaultRuntimeVersion,
		EmotionHalfLifeHours: round4(clampRange(4, 32, base.EmotionHalfLifeHours+stress*4.5-recovery*3)),
		MoodHalfLifeHours:    round4(clampRange(6, 44, base.MoodHalfLifeHours+stress*4+fatigue*2-recovery*3.5)),
		StressHalfLifeHours:  round4(clampRange(4, 36, base.StressHalfLifeHours+stress*5+fatigue*2.5-recovery*4)),
		NeedHalfLifeHours:    round4(clampRange(2, 24, base.NeedHalfLifeHours+fatigue*3-recovery*2)),
		MinRecoveryRate:      round4(minRecovery),
		MaxRecoveryRate:      round4(maxRecovery),
		ResilienceBias:       round4(clampRange(0.05, 0.98, base.ResilienceBias-stress*0.08-fatigue*0.04+recovery*0.08)),
	}
}

func modulateBehavior(base BehaviorProfile, influence RuntimeInfluence) BehaviorProfile {
	stress := influence.StressImpact
	fatigue := influence.FatigueImpact
	pressure := influence.Pressure
	recovery := influence.RecoveryImpact
	return BehaviorProfile{
		Version:             DefaultRuntimeVersion,
		InitiateWeight:      round4(clampRange(0.05, 0.98, base.InitiateWeight-fatigue*0.16-stress*0.08+recovery*0.05)),
		DirectWeight:        round4(clampRange(0.05, 0.98, base.DirectWeight-fatigue*0.08+stress*0.04)),
		HumorWeight:         round4(clampRange(0.01, 0.95, base.HumorWeight-fatigue*0.22-stress*0.08+recovery*0.07)),
		ConflictAvoidWeight: round4(clampRange(0.05, 0.98, base.ConflictAvoidWeight+pressure*0.18-recovery*0.05)),
		SupportWeight:       round4(clampRange(0.05, 0.98, base.SupportWeight-stress*0.08-fatigue*0.08+recovery*0.08)),
		InitiationThreshold: round4(clampRange(0.05, 0.95, base.InitiationThreshold+stress*0.1+fatigue*0.12-recovery*0.06)),
	}
}

func modulateExpression(base ExpressionPolicy, influence RuntimeInfluence) ExpressionPolicy {
	penalty := influence.ExpressionPenalty
	stress := influence.StressImpact
	fatigue := influence.FatigueImpact
	recovery := influence.RecoveryImpact
	minChars := maxInt(12, int(math.Round(float64(base.MinReplyChars)*(1-penalty*0.2+recovery*0.08))))
	maxChars := maxInt(minChars, int(math.Round(float64(base.MaxReplyChars)*(1-penalty*0.28+recovery*0.1))))
	maxSentences := maxInt(base.MinSentences, int(math.Round(float64(base.MaxSentences)-fatigue*2-stress+recovery)))

	return ExpressionPolicy{
		Version:             DefaultRuntimeVersion,
		MinReplyChars:       minChars,
		MaxReplyChars:       minInt(260, maxChars),
		MinSentences:        base.MinSentences,
		MaxSentences:        maxSentences,
		ShortSentenceBias:   round4(clampRange(0.1, 0.98, base.ShortSentenceBias+penalty*0.18)),
		Warmth:              round4(clampRange(0.05, 0.98, base.Warmth-stress*0.08-fatigue*0.05+recovery*0.05)),
		Rationality:         round4(clampRange(0.05, 0.98, base.Rationality-stress*0.12-fatigue*0.05+recovery*0.06)),
		Teasing:             round4(clampRange(0.01, 0.95, base.Teasing-fatigue*0.14-stress*0.08+recovery*0.04)),
		Intimacy:            round4(clampRange(0.01, 0.95, base.Intimacy-stress*0.09-fatigue*0.05+recovery*0.04)),
		SuggestionBias:      round4(clampRange(0.03, 0.95, base.SuggestionBias-fatigue*0.05+recovery*0.03)),
		EmotionalDisclosure: round4(clampRange(0.02, 0.95, base.EmotionalDisclosure-stress*0.1-fatigue*0.05+recovery*0.04)),
		ForbiddenStyles:     append([]string{}, base.ForbiddenStyles...),
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
