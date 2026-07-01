package affect

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const defaultPersonalityVersion = "affect-personality-ref-v1"

func DefaultState(now time.Time) AffectState {
	return AffectState{
		Version:   StateVersionV1,
		Emotion:   EmotionState{UpdatedAt: now},
		Mood:      MoodState{UpdatedAt: now},
		Stress:    0,
		UpdatedAt: now,
	}
}

func ComputeNextState(input EngineInput) EngineOutput {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	personality := normalizePersonality(input.Personality)
	budget := normalizeBudget(input.Budget)
	current := normalizeState(input.Current, now)
	appraisal := normalizeAppraisal(input.Appraisal)
	decayed, recoveryFactor, elapsedHours, diagnostics := decayState(current, personality, now)

	confidence := clampRange(0, 1, appraisal.Confidence)
	intensity := clampRange(0, 1, appraisal.Intensity)
	valence := clampRange(-1, 1, appraisal.Valence)
	arousal := clampRange(0, 1, appraisal.Arousal)
	social := clampRange(0, 1, appraisal.SocialRelevance)
	boundaryThreat := clampRange(0, 1, appraisal.BoundaryThreat)
	expectationGap := clampRange(0, 1, appraisal.ExpectationGap)
	control := clampRange(-1, 1, appraisal.Control)
	sensitivity := personality.Sensitivity
	stability := personality.Stability
	moodStickiness := personality.MoodStickiness
	confidenceBias := personality.ConfidenceBias
	controlBias := personality.ControlBias

	positiveDelta := 0.0
	if valence > 0 {
		positiveDelta = valence * confidence * intensity * (0.34 + social*0.18 + (1-sensitivity)*0.06)
	}

	negativeDelta := 0.0
	if valence < 0 {
		negativeDelta = -valence * confidence * intensity * (0.32 + sensitivity*0.24 + boundaryThreat*0.14 + expectationGap*0.12)
	}

	arousalDelta := arousal * confidence * intensity * (0.22 + sensitivity*0.16 + (1-stability)*0.12)
	if valence < 0 {
		arousalDelta += (-valence) * 0.06
	}

	dominanceDelta := 0.0
	dominanceBase := control * confidence * intensity * (0.18 + controlBias*0.14 + confidenceBias*0.06 + (1-sensitivity)*0.06)
	if valence > 0 && control > 0 {
		dominanceDelta = dominanceBase * (1 + valence*0.15)
	} else if valence < 0 && control < 0 {
		dominanceDelta = -(-dominanceBase) * (1 + (-valence)*0.12)
	} else if valence < 0 && control > 0 {
		dominanceDelta = dominanceBase * (1 - (-valence)*0.2)
	} else {
		dominanceDelta = dominanceBase
	}
	if boundaryThreat > 0.4 {
		dominanceDelta -= boundaryThreat * 0.08
	}
	if expectationGap > 0.5 {
		dominanceDelta -= expectationGap * 0.04
	}

	moodValenceDelta := valence * confidence * intensity * (0.12 + (1-moodStickiness)*0.18 + social*0.05)
	moodTensionDelta := ((arousal * 0.45) + (negativeDelta * 0.35) + (boundaryThreat * 0.2)) * (0.18 + sensitivity*0.12)
	stressDelta := ((negativeDelta * 0.5) + (boundaryThreat * 0.28) + (expectationGap * 0.16)) * confidence

	delta := AffectDelta{
		EmotionPositive: clampSigned(positiveDelta, budget.MaxEmotionDelta),
		EmotionNegative: clampSigned(negativeDelta, budget.MaxEmotionDelta),
		Dominance:       clampSigned(dominanceDelta, budget.MaxDominanceDelta),
		EmotionArousal:  clampSigned(arousalDelta, budget.MaxEmotionDelta),
		MoodValence:     clampSigned(moodValenceDelta, budget.MaxMoodDelta),
		MoodTension:     clampSigned(moodTensionDelta, budget.MaxMoodDelta),
		Stress:          clampSigned(stressDelta, budget.MaxStressDelta),
	}

	if delta.EmotionPositive != positiveDelta || delta.EmotionNegative != negativeDelta || delta.Dominance != dominanceDelta || delta.EmotionArousal != arousalDelta || delta.MoodValence != moodValenceDelta || delta.MoodTension != moodTensionDelta || delta.Stress != stressDelta {
		diagnostics = append(diagnostics, "budget_clamped")
	}
	if valence > 0 {
		diagnostics = append(diagnostics, "positive_appraisal")
	}
	if valence < 0 {
		diagnostics = append(diagnostics, "negative_appraisal")
	}
	if boundaryThreat >= 0.6 {
		diagnostics = append(diagnostics, "boundary_threat_elevated")
	}
	if dominanceDelta > 0 {
		diagnostics = append(diagnostics, "dominance_increased")
	} else if dominanceDelta < 0 {
		diagnostics = append(diagnostics, "dominance_decreased")
	}

	pleasure := clampRange(0, 1, 0.5+decayed.Emotion.Positive*0.5-decayed.Emotion.Negative*0.4)
	newDominance := round4(clampRange(0, 1, decayed.Emotion.Dominance+delta.Dominance))
	newPleasure := round4(clampRange(0, 1, pleasure))
	newArousal := round4(clampRange(0, 1, decayed.Emotion.Arousal+delta.EmotionArousal))
	padLabel := PADLabel(newPleasure, newArousal, newDominance)

	next := AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{
			Positive:    round4(clampRange(0, 1, decayed.Emotion.Positive+delta.EmotionPositive)),
			Negative:    round4(clampRange(0, 1, decayed.Emotion.Negative+delta.EmotionNegative)),
			Arousal:     newArousal,
			Dominance:   newDominance,
			LastEventID: appraisal.EventID,
			UpdatedAt:   now,
		},
		Mood: MoodState{
			PAD:       padLabel,
			Valence:   round4(clampRange(-1, 1, decayed.Mood.Valence+delta.MoodValence)),
			Tension:   round4(clampRange(0, 1, decayed.Mood.Tension+delta.MoodTension)),
			UpdatedAt: now,
		},
		Stress:    round4(clampRange(0, 1, decayed.Stress+delta.Stress)),
		UpdatedAt: now,
	}

	if next.Emotion.Positive == 0 && next.Emotion.Negative == 0 && next.Emotion.Arousal == 0 && next.Emotion.Dominance == 0 && next.Mood.Valence == 0 && next.Mood.Tension == 0 && next.Stress == 0 {
		diagnostics = append(diagnostics, "baseline_state")
	}

	if padLabel != "" {
		diagnostics = append(diagnostics, "pad_"+padLabel)
	}

	sort.Strings(diagnostics)
	return EngineOutput{
		State: next,
		Delta: roundDelta(delta),
		Audit: AffectAudit{
			RecoveryFactor: round4(recoveryFactor),
			ElapsedHours:   round4(elapsedHours),
			Budget:         budget,
			Diagnostics:    uniqueStrings(diagnostics),
		},
	}
}

func PADLabel(pleasure, arousal, dominance float64) string {
	p := "neutral"
	a := "neutral"
	d := "neutral"
	if pleasure >= 0.55 {
		p = "pleasant"
	} else if pleasure <= 0.35 {
		p = "unpleasant"
	}
	if arousal >= 0.55 {
		a = "aroused"
	} else if arousal <= 0.35 {
		a = "calm"
	}
	if dominance >= 0.55 {
		d = "dominant"
	} else if dominance <= 0.35 {
		d = "submissive"
	}
	return fmt.Sprintf("%s_%s_%s", p, a, d)
}

func normalizePersonality(input PersonalityReference) PersonalityReference {
	version := input.Version
	if version == "" {
		version = defaultPersonalityVersion
	}
	return PersonalityReference{
		Version:        version,
		Sensitivity:    clampRange(0, 1, fallbackUnit(input.Sensitivity, 0.5)),
		Stability:      clampRange(0, 1, fallbackUnit(input.Stability, 0.55)),
		RecoveryBias:   clampRange(0, 1, fallbackUnit(input.RecoveryBias, 0.5)),
		ConfidenceBias: clampRange(0, 1, fallbackUnit(input.ConfidenceBias, 0.5)),
		ControlBias:    clampRange(0, 1, fallbackUnit(input.ControlBias, 0.5)),
		MoodStickiness: clampRange(0, 1, fallbackUnit(input.MoodStickiness, 0.6)),
		Boundary:       clampRange(0, 1, fallbackUnit(input.Boundary, 0.55)),
	}
}

func normalizeBudget(input ChangeBudget) ChangeBudget {
	emotion := input.MaxEmotionDelta
	mood := input.MaxMoodDelta
	dominance := input.MaxDominanceDelta
	stress := input.MaxStressDelta
	if emotion <= 0 {
		emotion = 0.3
	}
	if mood <= 0 {
		mood = 0.18
	}
	if dominance <= 0 {
		dominance = 0.2
	}
	if stress <= 0 {
		stress = 0.16
	}
	return ChangeBudget{
		MaxEmotionDelta:   round4(clampRange(0.02, 1, emotion)),
		MaxMoodDelta:      round4(clampRange(0.02, 1, mood)),
		MaxDominanceDelta: round4(clampRange(0.02, 1, dominance)),
		MaxStressDelta:    round4(clampRange(0.02, 1, stress)),
	}
}

func normalizeState(input AffectState, now time.Time) AffectState {
	if input.Version == "" {
		input.Version = StateVersionV1
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}
	if input.Emotion.UpdatedAt.IsZero() {
		input.Emotion.UpdatedAt = input.UpdatedAt
	}
	if input.Mood.UpdatedAt.IsZero() {
		input.Mood.UpdatedAt = input.UpdatedAt
	}
	input.Emotion.Positive = clampRange(0, 1, input.Emotion.Positive)
	input.Emotion.Negative = clampRange(0, 1, input.Emotion.Negative)
	input.Emotion.Arousal = clampRange(0, 1, input.Emotion.Arousal)
	input.Emotion.Dominance = clampRange(0, 1, input.Emotion.Dominance)
	input.Mood.Valence = clampRange(-1, 1, input.Mood.Valence)
	input.Mood.Tension = clampRange(0, 1, input.Mood.Tension)
	input.Stress = clampRange(0, 1, input.Stress)
	return input
}

func normalizeAppraisal(input EventAppraisal) EventAppraisal {
	if input.Confidence == 0 {
		input.Confidence = 1
	}
	if input.Intensity == 0 {
		input.Intensity = 1
	}
	return EventAppraisal{
		EventID:         input.EventID,
		OccurredAt:      input.OccurredAt,
		Valence:         clampRange(-1, 1, input.Valence),
		Arousal:         clampRange(0, 1, input.Arousal),
		SocialRelevance: clampRange(0, 1, input.SocialRelevance),
		BoundaryThreat:  clampRange(0, 1, input.BoundaryThreat),
		Confidence:      clampRange(0, 1, input.Confidence),
		Control:         clampRange(-1, 1, input.Control),
		ExpectationGap:  clampRange(0, 1, input.ExpectationGap),
		Intensity:       clampRange(0, 1, input.Intensity),
	}
}

func decayState(current AffectState, personality PersonalityReference, now time.Time) (AffectState, float64, float64, []string) {
	elapsedHours := 0.0
	if now.After(current.UpdatedAt) {
		elapsedHours = now.Sub(current.UpdatedAt).Hours()
	}
	if elapsedHours <= 0 {
		return current, 0, 0, nil
	}

	emotionHalfLife := clampRange(1.5, 16, 5.5+(1-personality.Stability)*6+personality.Sensitivity*2.5)
	moodHalfLife := clampRange(6, 72, 18+personality.MoodStickiness*28+(1-personality.RecoveryBias)*8)
	stressHalfLife := clampRange(3, 36, 9+(1-personality.Stability)*12+(1-personality.RecoveryBias)*6)
	dominanceHalfLife := clampRange(2, 24, 8+personality.ControlBias*8+(1-personality.Stability)*4)
	recoveryBoost := 0.85 + personality.RecoveryBias*0.3

	emotionDecay := decayFactor(elapsedHours, emotionHalfLife, recoveryBoost)
	moodDecay := decayFactor(elapsedHours, moodHalfLife, recoveryBoost)
	stressDecay := decayFactor(elapsedHours, stressHalfLife, recoveryBoost)
	dominanceDecay := decayFactor(elapsedHours, dominanceHalfLife, recoveryBoost)

	return AffectState{
		Version: current.Version,
		Emotion: EmotionState{
			Positive:    round4(current.Emotion.Positive * emotionDecay),
			Negative:    round4(current.Emotion.Negative * emotionDecay),
			Arousal:     round4(current.Emotion.Arousal * emotionDecay),
			Dominance:   round4(current.Emotion.Dominance * dominanceDecay),
			LastEventID: current.Emotion.LastEventID,
			UpdatedAt:   now,
		},
		Mood: MoodState{
			PAD:       current.Mood.PAD,
			Valence:   round4(current.Mood.Valence * moodDecay),
			Tension:   round4(current.Mood.Tension * moodDecay),
			UpdatedAt: now,
		},
		Stress:    round4(current.Stress * stressDecay),
		UpdatedAt: now,
	}, 1 - emotionDecay, elapsedHours, []string{"decay_applied"}
}

func decayFactor(elapsedHours, halfLife, recoveryBoost float64) float64 {
	if elapsedHours <= 0 || halfLife <= 0 {
		return 1
	}
	value := math.Exp(-math.Ln2 * elapsedHours * recoveryBoost / halfLife)
	return clampRange(0, 1, value)
}

func roundDelta(delta AffectDelta) AffectDelta {
	return AffectDelta{
		EmotionPositive: round4(delta.EmotionPositive),
		EmotionNegative: round4(delta.EmotionNegative),
		Dominance:       round4(delta.Dominance),
		EmotionArousal:  round4(delta.EmotionArousal),
		MoodValence:     round4(delta.MoodValence),
		MoodTension:     round4(delta.MoodTension),
		Stress:          round4(delta.Stress),
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fallbackUnit(value, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}

func clampSigned(value, limit float64) float64 {
	if value > limit {
		return limit
	}
	if value < -limit {
		return -limit
	}
	return value
}

func clampRange(minimum, maximum, value float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}