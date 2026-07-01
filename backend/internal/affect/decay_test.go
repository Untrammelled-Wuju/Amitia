package affect

import (
	"testing"
	"time"
)

func TestStateRecoversOverTime(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	personality := PersonalityReference{Sensitivity: 0.5, Stability: 0.5, RecoveryBias: 0.5, MoodStickiness: 0.5, Boundary: 0.5}
	initial := AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{Positive: 0.3, Negative: 0.2, Arousal: 0.4, Dominance: 0.35, UpdatedAt: now},
		Mood:    MoodState{Valence: 0.1, Tension: 0.15, UpdatedAt: now},
		Stress:  0.25,
		UpdatedAt: now,
	}

	later := now.Add(24 * time.Hour)
	output := ComputeNextState(EngineInput{
		Current:     initial,
		Personality: personality,
		Appraisal:   EventAppraisal{EventID: "recovery-check"},
		Now:         later,
	})
	if output.State.Emotion.Positive >= initial.Emotion.Positive {
		t.Fatalf("expected emotion to decay after 24h: %#v", output.State.Emotion)
	}
	if output.State.Stress >= initial.Stress {
		t.Fatalf("expected stress to decay: %#v", output.State.Stress)
	}
	if !contains(output.Audit.Diagnostics, "decay_applied") {
		t.Fatalf("expected decay diagnostic: %#v", output.Audit.Diagnostics)
	}
}

func TestStrongNegativeUnlocksBoundaryAndTension(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	output := ComputeNextState(EngineInput{
		Now: now,
		Personality: PersonalityReference{
			Sensitivity: 0.7, Stability: 0.4, RecoveryBias: 0.35,
			MoodStickiness: 0.6, Boundary: 0.65,
		},
		Appraisal: EventAppraisal{
			EventID: "severe-conflict", Valence: -0.95, Arousal: 0.85,
			BoundaryThreat: 0.9, ExpectationGap: 0.7,
			SocialRelevance: 0.9, Confidence: 0.95, Intensity: 0.9,
		},
	})
	if output.State.Emotion.Negative < 0.1 {
		t.Fatalf("expected elevated negative emotion: %#v", output.State.Emotion)
	}
	if output.State.Mood.Tension < 0.1 {
		t.Fatalf("expected elevated tension: %#v", output.State.Mood)
	}
	if output.State.Stress < 0.1 {
		t.Fatalf("expected elevated stress: %#v", output.State.Stress)
	}
}