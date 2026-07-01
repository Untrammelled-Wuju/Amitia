package affect

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestComputeNextStateDefaultsToBaselineState(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	output := ComputeNextState(EngineInput{
		Now: now,
		Appraisal: EventAppraisal{
			EventID: "evt-neutral",
		},
	})

	if output.State.Version != StateVersionV1 {
		t.Fatalf("unexpected version: %v", output.State.Version)
	}
	if output.State.Emotion.Positive != 0 || output.State.Emotion.Negative != 0 || output.State.Emotion.Arousal != 0 || output.State.Emotion.Dominance != 0 {
		t.Fatalf("expected zero emotion baseline: %#v", output.State.Emotion)
	}
	if output.State.Mood.Valence != 0 || output.State.Mood.Tension != 0 || output.State.Stress != 0 {
		t.Fatalf("expected zero mood/stress baseline: %#v %#v", output.State.Mood, output.State.Stress)
	}
	if !contains(output.Audit.Diagnostics, "baseline_state") {
		t.Fatalf("expected baseline diagnostic: %#v", output.Audit.Diagnostics)
	}
}

func TestComputeNextStatePositiveAndNegativeAppraisalShiftInExpectedDirection(t *testing.T) {
	now := time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC)
	personality := PersonalityReference{
		Sensitivity:    0.62,
		Stability:      0.58,
		RecoveryBias:   0.5,
		MoodStickiness: 0.55,
		Boundary:       0.48,
	}

	positive := ComputeNextState(EngineInput{
		Now:         now,
		Personality: personality,
		Appraisal: EventAppraisal{
			EventID:         "evt-positive",
			Valence:         0.9,
			Arousal:         0.45,
			SocialRelevance: 0.7,
			Confidence:      0.9,
			Intensity:       0.85,
		},
	})
	if positive.State.Emotion.Positive <= 0 || positive.State.Mood.Valence <= 0 {
		t.Fatalf("expected positive shift: %#v", positive.State)
	}

	negative := ComputeNextState(EngineInput{
		Now:         now,
		Personality: personality,
		Appraisal: EventAppraisal{
			EventID:         "evt-negative",
			Valence:         -0.85,
			Arousal:         0.7,
			SocialRelevance: 0.8,
			BoundaryThreat:  0.65,
			ExpectationGap:  0.6,
			Confidence:      0.92,
			Intensity:       0.9,
		},
	})
	if negative.State.Emotion.Negative <= 0 || negative.State.Mood.Tension <= 0 || negative.State.Stress <= 0 {
		t.Fatalf("expected negative/tension shift: %#v", negative.State)
	}
	if !contains(negative.Audit.Diagnostics, "boundary_threat_elevated") {
		t.Fatalf("expected conservative boundary diagnostic: %#v", negative.Audit.Diagnostics)
	}
}

func TestComputeNextStateAppliesBudgetClamp(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	output := ComputeNextState(EngineInput{
		Now: now,
		Personality: PersonalityReference{
			Sensitivity:    1,
			Stability:      0.1,
			RecoveryBias:   0.1,
			MoodStickiness: 0.9,
			Boundary:       1,
		},
		Budget: ChangeBudget{
			MaxEmotionDelta:   0.08,
			MaxMoodDelta:      0.05,
			MaxDominanceDelta: 0.04,
			MaxStressDelta:    0.04,
		},
		Appraisal: EventAppraisal{
			EventID:         "evt-budget",
			Valence:         -1,
			Arousal:         1,
			SocialRelevance: 1,
			BoundaryThreat:  1,
			ExpectationGap:  1,
			Confidence:      1,
			Intensity:       1,
		},
	})

	if output.Delta.EmotionNegative != 0.08 || output.Delta.MoodTension != 0.05 || output.Delta.Stress != 0.04 || output.Delta.Dominance != -0.04 {
		t.Fatalf("expected exact budget clamp: %#v", output.Delta)
	}
	if !contains(output.Audit.Diagnostics, "budget_clamped") {
		t.Fatalf("expected budget clamp diagnostic: %#v", output.Audit.Diagnostics)
	}
}

func TestComputeNextStateAppliesTimeDecayBeforeNeutralInput(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	current := AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{
			Positive:  0.7,
			Negative:  0.4,
			Arousal:   0.6,
			Dominance: 0.5,
			UpdatedAt: now.Add(-24 * time.Hour),
		},
		Mood: MoodState{
			Valence:   0.5,
			Tension:   0.4,
			UpdatedAt: now.Add(-24 * time.Hour),
		},
		Stress:    0.55,
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	output := ComputeNextState(EngineInput{
		Now: now,
		Personality: PersonalityReference{
			Sensitivity:    0.45,
			Stability:      0.72,
			RecoveryBias:   0.75,
			MoodStickiness: 0.65,
			Boundary:       0.5,
		},
		Current: current,
		Appraisal: EventAppraisal{
			EventID:    "evt-neutral",
			Confidence: 1,
			Intensity:  0,
		},
	})

	if output.State.Emotion.Positive >= current.Emotion.Positive || output.State.Mood.Valence >= current.Mood.Valence || output.State.Stress >= current.Stress {
		t.Fatalf("expected decayed state: current=%#v next=%#v", current, output.State)
	}
	if !contains(output.Audit.Diagnostics, "decay_applied") {
		t.Fatalf("expected decay diagnostic: %#v", output.Audit.Diagnostics)
	}
}

func TestComputeNextStateIsStableForRepeatedInput(t *testing.T) {
	now := time.Date(2026, 7, 1, 13, 0, 0, 0, time.UTC)
	input := EngineInput{
		Now: now,
		Current: AffectState{
			Version: StateVersionV1,
			Emotion: EmotionState{
				Positive:  0.18,
				Negative:  0.11,
				Arousal:   0.2,
				Dominance: 0.15,
				UpdatedAt: now.Add(-2 * time.Hour),
			},
			Mood: MoodState{
				Valence:   0.09,
				Tension:   0.17,
				UpdatedAt: now.Add(-2 * time.Hour),
			},
			Stress:    0.16,
			UpdatedAt: now.Add(-2 * time.Hour),
		},
		Personality: PersonalityReference{
			Sensitivity:    0.57,
			Stability:      0.61,
			RecoveryBias:   0.48,
			MoodStickiness: 0.59,
			Boundary:       0.52,
		},
		Appraisal: EventAppraisal{
			EventID:         "evt-repeat",
			Valence:         -0.35,
			Arousal:         0.42,
			SocialRelevance: 0.63,
			BoundaryThreat:  0.24,
			ExpectationGap:  0.2,
			Confidence:      0.88,
			Intensity:       0.67,
		},
	}

	first := ComputeNextState(input)
	second := ComputeNextState(input)
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
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}