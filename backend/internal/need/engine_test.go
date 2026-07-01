package need

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUpdateNeedsUsesDefaultSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 1, 14, 0, 0, 0, time.UTC)
	result := UpdateNeeds(UpdateInput{Now: now})
	if result.Before.States[NeedKindConnection].Level != DefaultSnapshot(now).States[NeedKindConnection].Level {
		t.Fatalf("expected default snapshot, got %#v", result.Before)
	}
	if !contains(result.Audit.Diagnostics, "default_snapshot") {
		t.Fatalf("expected default snapshot diagnostic: %#v", result.Audit.Diagnostics)
	}
}

func TestSignalsShiftTargetNeeds(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 0, 0, 0, time.UTC)
	current := DefaultSnapshot(now)
	result := UpdateNeeds(UpdateInput{
		Now:     now,
		Current: current,
		Signals: []NeedSignal{
			{Kind: NeedKindReassurance, Pressure: 0.9, Confidence: 0.9},
			{Kind: NeedKindConnection, Pressure: 0.8, Confidence: 0.85},
			{Kind: NeedKindAutonomy, Relief: 0.75, Confidence: 0.9},
		},
		Personality: PersonalityRef{
			Version:          "need-personality-v1",
			Sensitivity:      0.68,
			Stability:        0.52,
			RecoveryBias:     0.5,
			AttachmentBias:   0.74,
			BoundaryStrength: 0.64,
		},
	})
	if result.After.States[NeedKindReassurance].Level <= current.States[NeedKindReassurance].Level {
		t.Fatalf("expected reassurance need increase")
	}
	if result.After.States[NeedKindConnection].Level <= current.States[NeedKindConnection].Level {
		t.Fatalf("expected connection need increase")
	}
	if result.After.States[NeedKindAutonomy].Level >= current.States[NeedKindAutonomy].Level {
		t.Fatalf("expected autonomy need decrease")
	}
}

func TestDefaultSnapshotIncludesExpressionAndNoveltyNeeds(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 30, 0, 0, time.UTC)
	snapshot := DefaultSnapshot(now)
	if _, ok := snapshot.States[NeedKindExpression]; !ok {
		t.Fatalf("expected expression need in default snapshot")
	}
	if _, ok := snapshot.States[NeedKindNovelty]; !ok {
		t.Fatalf("expected novelty need in default snapshot")
	}
}

func TestExpressionAndNoveltySignalsRemainBounded(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 45, 0, 0, time.UTC)
	current := DefaultSnapshot(now)
	result := UpdateNeeds(UpdateInput{
		Now:     now,
		Current: current,
		Signals: []NeedSignal{
			{Kind: NeedKindExpression, Pressure: 1, Confidence: 1},
			{Kind: NeedKindNovelty, Pressure: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			Sensitivity:      0.7,
			Stability:        0.35,
			RecoveryBias:     0.5,
			AttachmentBias:   0.55,
			BoundaryStrength: 0.5,
		},
		Budget: ChangeBudget{
			MaxLevelDelta: 0.08,
			MaxTrendDelta: 0.05,
		},
	})
	if result.After.States[NeedKindExpression].Level <= current.States[NeedKindExpression].Level {
		t.Fatalf("expected expression need increase")
	}
	if result.After.States[NeedKindNovelty].Level <= current.States[NeedKindNovelty].Level {
		t.Fatalf("expected novelty need increase")
	}
	if result.Delta[NeedKindExpression].Level > 0.08 || result.Delta[NeedKindNovelty].Level > 0.08 {
		t.Fatalf("expected new need deltas to respect budget: %#v", result.Delta)
	}
}

func TestBudgetClampLimitsNeedDelta(t *testing.T) {
	now := time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC)
	result := UpdateNeeds(UpdateInput{
		Now: now,
		Signals: []NeedSignal{
			{Kind: NeedKindClarity, Pressure: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			Sensitivity:      1,
			Stability:        0,
			RecoveryBias:     0,
			AttachmentBias:   0.5,
			BoundaryStrength: 0.5,
		},
		Budget: ChangeBudget{
			MaxLevelDelta: 0.03,
			MaxTrendDelta: 0.02,
		},
	})
	if result.Delta[NeedKindClarity].Level != 0.03 || result.Delta[NeedKindClarity].Trend != 0.02 {
		t.Fatalf("expected clamp result, got %#v", result.Delta[NeedKindClarity])
	}
	if !contains(result.Audit.Diagnostics, "budget_clamped") {
		t.Fatalf("expected budget clamp diagnostic: %#v", result.Audit.Diagnostics)
	}
}

func TestUpdateNeedsAppliesDecayTowardBaseline(t *testing.T) {
	now := time.Date(2026, 7, 2, 16, 0, 0, 0, time.UTC)
	before := DefaultSnapshot(now.Add(-24 * time.Hour))
	state := before.States[NeedKindRest]
	state.Level = 0.82
	state.Trend = 0.6
	before.States[NeedKindRest] = state
	before.UpdatedAt = now.Add(-24 * time.Hour)

	result := UpdateNeeds(UpdateInput{
		Now:     now,
		Current: before,
		Personality: PersonalityRef{
			Sensitivity:      0.42,
			Stability:        0.7,
			RecoveryBias:     0.78,
			AttachmentBias:   0.48,
			BoundaryStrength: 0.58,
		},
	})
	if result.After.States[NeedKindRest].Level >= before.States[NeedKindRest].Level {
		t.Fatalf("expected decay toward baseline")
	}
	if result.After.States[NeedKindRest].Trend >= before.States[NeedKindRest].Trend {
		t.Fatalf("expected trend decay")
	}
	if !contains(result.Audit.Diagnostics, "decay_applied") {
		t.Fatalf("expected decay diagnostic: %#v", result.Audit.Diagnostics)
	}
}

func TestUpdateNeedsMarksSaturatedNeedAtUpperBound(t *testing.T) {
	now := time.Date(2026, 7, 1, 16, 30, 0, 0, time.UTC)
	current := DefaultSnapshot(now)
	state := current.States[NeedKindConnection]
	state.Level = 0.87
	state.Trend = 0.04
	current.States[NeedKindConnection] = state

	result := UpdateNeeds(UpdateInput{
		Now:     now,
		Current: current,
		Signals: []NeedSignal{
			{Kind: NeedKindConnection, Pressure: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			Sensitivity:      0.55,
			Stability:        0.45,
			RecoveryBias:     0.4,
			AttachmentBias:   1,
			BoundaryStrength: 0.5,
		},
		Budget: ChangeBudget{
			MaxLevelDelta: 0.14,
			MaxTrendDelta: 0.1,
		},
	})

	if !result.After.States[NeedKindConnection].Saturated {
		t.Fatalf("expected connection need saturated: %#v", result.After.States[NeedKindConnection])
	}
	if !contains(result.Audit.Diagnostics, "need_saturated:connection") {
		t.Fatalf("expected saturation diagnostic: %#v", result.Audit.Diagnostics)
	}
}

func TestUpdateNeedsClearsSaturationAfterRelief(t *testing.T) {
	now := time.Date(2026, 7, 1, 16, 45, 0, 0, time.UTC)
	current := DefaultSnapshot(now)
	state := current.States[NeedKindReassurance]
	state.Level = 0.92
	state.Trend = 0.05
	state.Saturated = true
	current.States[NeedKindReassurance] = state

	result := UpdateNeeds(UpdateInput{
		Now:     now,
		Current: current,
		Signals: []NeedSignal{
			{Kind: NeedKindReassurance, Relief: 1, Confidence: 1},
		},
		Personality: PersonalityRef{
			Sensitivity:      0.6,
			Stability:        0.6,
			RecoveryBias:     0.7,
			AttachmentBias:   0.7,
			BoundaryStrength: 0.5,
		},
		Budget: ChangeBudget{
			MaxLevelDelta: 0.14,
			MaxTrendDelta: 0.1,
		},
	})

	if result.After.States[NeedKindReassurance].Saturated {
		t.Fatalf("expected relief to clear saturation: %#v", result.After.States[NeedKindReassurance])
	}
	if result.After.States[NeedKindReassurance].Level >= current.States[NeedKindReassurance].Level {
		t.Fatalf("expected relief to lower need level")
	}
}

func TestRepeatedInputProducesStableOutput(t *testing.T) {
	now := time.Date(2026, 7, 1, 17, 0, 0, 0, time.UTC)
	input := UpdateInput{
		Now:     now,
		Current: DefaultSnapshot(now.Add(-3 * time.Hour)),
		Signals: []NeedSignal{
			{Kind: NeedKindConnection, Pressure: 0.55, Confidence: 0.9},
			{Kind: NeedKindClarity, Relief: 0.2, Pressure: 0.5, Confidence: 0.8},
		},
		Personality: PersonalityRef{
			Version:          "need-personality-v2",
			Sensitivity:      0.57,
			Stability:        0.61,
			RecoveryBias:     0.49,
			AttachmentBias:   0.62,
			BoundaryStrength: 0.53,
		},
	}
	first := UpdateNeeds(input)
	second := UpdateNeeds(input)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("expected stable output\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
