package psyche

import (
	"reflect"
	"testing"
)

func TestModulateRuntimeStressIncreasesPressure(t *testing.T) {
	profile := CompilePersonality(DefaultConfig())
	lowStress := 10.0
	highStress := 90.0

	low := ModulateRuntime(profile, RuntimeStateInput{Stress: &lowStress})
	high := ModulateRuntime(profile, RuntimeStateInput{Stress: &highStress})

	if high.Influence.Pressure <= low.Influence.Pressure {
		t.Fatalf("pressure did not increase: low=%#v high=%#v", low.Influence, high.Influence)
	}
	if high.Internal.StableCore.EmotionStability >= low.Internal.StableCore.EmotionStability {
		t.Fatalf("emotion stability did not decrease: low=%#v high=%#v", low.Internal.StableCore, high.Internal.StableCore)
	}
	if high.Appraisal.AmplificationCap <= low.Appraisal.AmplificationCap {
		t.Fatalf("amplification did not increase: low=%#v high=%#v", low.Appraisal, high.Appraisal)
	}
}

func TestModulateRuntimeFatigueReducesExpressionAndInitiative(t *testing.T) {
	profile := CompilePersonality(DefaultConfig())
	lowFatigue := 5.0
	highFatigue := 95.0

	rested := ModulateRuntime(profile, RuntimeStateInput{Fatigue: &lowFatigue})
	tired := ModulateRuntime(profile, RuntimeStateInput{Fatigue: &highFatigue})

	if tired.Behavior.InitiateWeight >= rested.Behavior.InitiateWeight {
		t.Fatalf("initiative did not decrease: rested=%#v tired=%#v", rested.Behavior, tired.Behavior)
	}
	if tired.Expression.MaxReplyChars >= rested.Expression.MaxReplyChars {
		t.Fatalf("reply size did not decrease: rested=%#v tired=%#v", rested.Expression, tired.Expression)
	}
	if tired.Internal.Growth.Humor >= rested.Internal.Growth.Humor {
		t.Fatalf("humor did not decrease: rested=%#v tired=%#v", rested.Internal.Growth, tired.Internal.Growth)
	}
}

func TestModulateRuntimeClampsBoundaries(t *testing.T) {
	profile := CompilePersonality(DefaultConfig())
	negative := -40.0
	huge := 180.0
	hours := 120.0

	result := ModulateRuntime(profile, RuntimeStateInput{
		Stress:        &huge,
		Fatigue:       &negative,
		Arousal:       &huge,
		MoodPressure:  &negative,
		SocialLoad:    &huge,
		RecoveryHours: &hours,
	})

	if result.State.Stress != 1 || result.State.Fatigue != 0 || result.State.RecoveryHours != 72 {
		t.Fatalf("unexpected state clamp: %#v", result.State)
	}
	if result.Sources["stress"] != "user_clamped" || result.Sources["recoveryHours"] != "user_clamped" {
		t.Fatalf("unexpected sources: %#v", result.Sources)
	}
	if len(result.Diagnostics) != 6 {
		t.Fatalf("expected six clamp diagnostics, got %#v", result.Diagnostics)
	}
	assertRuntimeBounds(t, result)
}

func TestModulateRuntimeEmptyInputUsesStableDefaults(t *testing.T) {
	profile := CompilePersonality(DefaultConfig())

	first := ModulateRuntime(profile, RuntimeStateInput{})
	second := ModulateRuntime(profile, RuntimeStateInput{})

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("empty input is not stable\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.State.Stress != 0 || first.State.Fatigue != 0 || first.State.RecoveryHours != 0 {
		t.Fatalf("unexpected default state: %#v", first.State)
	}
	if first.Sources["stress"] != "default" || first.Sources["socialLoad"] != "default" {
		t.Fatalf("unexpected default sources: %#v", first.Sources)
	}
	assertRuntimeBounds(t, first)
}

func TestModulateRuntimeDeterministic(t *testing.T) {
	initiative := 82.0
	sensitivity := 70.0
	stability := 34.0
	profile := CompilePersonality(PersonalityConfig{
		Initiative:  &initiative,
		Sensitivity: &sensitivity,
		Stability:   &stability,
	})
	stress := 74.0
	fatigue := 66.0
	arousal := 52.0
	mood := 41.0
	social := 63.0
	recovery := 7.0
	input := RuntimeStateInput{
		Stress:        &stress,
		Fatigue:       &fatigue,
		Arousal:       &arousal,
		MoodPressure:  &mood,
		SocialLoad:    &social,
		RecoveryHours: &recovery,
	}

	first := ModulateRuntime(profile, input)
	second := ModulateRuntime(profile, input)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("runtime modulation is not deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}
	assertRuntimeBounds(t, first)
}

func assertRuntimeBounds(t *testing.T, result RuntimeModulation) {
	t.Helper()
	values := map[string]float64{
		"stressImpact":      result.Influence.StressImpact,
		"fatigueImpact":     result.Influence.FatigueImpact,
		"recoveryImpact":    result.Influence.RecoveryImpact,
		"regulation":        result.Influence.Regulation,
		"expressionPenalty": result.Influence.ExpressionPenalty,
		"pressure":          result.Influence.Pressure,
		"volatility":        result.Influence.Volatility,
		"initiative":        result.Internal.StableCore.SocialInitiative,
		"rejection":         result.Internal.StableCore.RejectionSensitivity,
		"humor":             result.Internal.Growth.Humor,
		"verbosity":         result.Internal.Situational.Verbosity,
	}
	for name, value := range values {
		if value < 0 || value > 1 {
			t.Fatalf("%s out of bounds: %#v", name, result)
		}
	}
	if result.Expression.MinReplyChars > result.Expression.MaxReplyChars {
		t.Fatalf("expression char bounds invalid: %#v", result.Expression)
	}
	if result.Recovery.MaxRecoveryRate < result.Recovery.MinRecoveryRate {
		t.Fatalf("recovery rate bounds invalid: %#v", result.Recovery)
	}
}
