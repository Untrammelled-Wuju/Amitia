package chat

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestBuildProactiveEmotionFromPsyche_NilRuntime(t *testing.T) {
	result := buildProactiveEmotionFromPsyche(nil)
	if result != "" {
		t.Errorf("nil runtime should return empty, got: %s", result)
	}
}

func TestBuildProactiveEmotionFromPsyche_NotReady(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{
		Context: interaction.ContextSnapshot{
			Psyche: interaction.SnapshotField[interaction.PsycheState]{
				Status: interaction.LoadStatusUnavailable,
			},
		},
	}
	result := buildProactiveEmotionFromPsyche(runtime)
	if result != "" {
		t.Errorf("not-ready psyche should return empty, got: %s", result)
	}
}

func TestBuildProactiveEmotionFromPsyche_NormalState(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{
		Context: interaction.ContextSnapshot{
			Psyche: interaction.FieldReady(interaction.PsycheState{
				Valence:     0.7,
				Arousal:     0.6,
				Dominance:   0.4,
				MoodValence: 0.5,
				MoodArousal: 0.3,
				Stress:      0.3,
				Fatigue:     0.15,
			}, "psyche", "v1"),
		},
	}
	result := buildProactiveEmotionFromPsyche(runtime)
	if result == "" {
		t.Fatal("should produce non-empty emotion string")
	}
	checks := []string{
		"\"valence\":0.70",
		"\"arousal\":0.60",
		"\"dominance\":0.40",
		"\"moodValence\":0.50",
		"\"moodArousal\":0.30",
		"压力：30",
		"精力：85",
		"情绪",
		"心情",
	}
	for _, c := range checks {
		if !strings.Contains(result, c) {
			t.Errorf("result should contain %q, got: %s", c, result)
		}
	}
}

func TestBuildProactiveEmotionFromPsyche_HighStress(t *testing.T) {
	runtime := &interaction.RuntimeAssembly{
		Context: interaction.ContextSnapshot{
			Psyche: interaction.FieldReady(interaction.PsycheState{
				Valence:     0.2,
				Arousal:     0.9,
				Dominance:   0.1,
				MoodValence: 0.1,
				MoodArousal: 0.8,
				Stress:      0.9,
				Fatigue:     0.6,
			}, "psyche", "v1"),
		},
	}
	result := buildProactiveEmotionFromPsyche(runtime)
	if result == "" {
		t.Fatal("should produce non-empty emotion string")
	}
	if !strings.Contains(result, "\"valence\":0.20") {
		t.Errorf("should reflect low valence, got: %s", result)
	}
	if !strings.Contains(result, "压力：90") {
		t.Errorf("should reflect high stress 90, got: %s", result)
	}
	if !strings.Contains(result, "精力：40") {
		t.Errorf("should reflect low energy 40, got: %s", result)
	}
}
