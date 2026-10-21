package psyche

import (
	"math"
	"time"

	"github.com/google/uuid"
)

func StateVersionV1() string {
	return "psyche-state-v1"
}

func NewPsycheState(characterID string) PsycheState {
	now := time.Now().UTC()
	return PsycheState{
		CharacterID:  characterID,
		Version:      StateVersionV1(),
		StateVersion: 1,
		Emotion: EmotionDimensions{
			Valence:   0.5,
			Arousal:   0.5,
			Dominance: 0.5,
		},
		Mood: MoodDimensions{
			MoodValence: 0.5,
			MoodArousal: 0.5,
		},
		Stress:    0,
		Energy:    0.7,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func ApplyEvent(state PsycheState, event PsycheEvent) PsycheState {
	state.Emotion.Valence = clampSignedDelta(state.Emotion.Valence, event.ValenceDelta)
	state.Emotion.Arousal = clampSignedDelta(state.Emotion.Arousal, event.ArousalDelta)
	state.Emotion.Dominance = clampSignedDelta(state.Emotion.Dominance, event.DominanceDelta)

	moodTransfer := math.Abs(event.ValenceDelta) * 0.1
	if event.ValenceDelta > 0 {
		state.Mood.MoodValence = clampSignedDelta(state.Mood.MoodValence, moodTransfer)
	} else if event.ValenceDelta < 0 {
		state.Mood.MoodValence = clampSignedDelta(state.Mood.MoodValence, -moodTransfer)
	}

	moodArousalTransfer := math.Abs(event.ArousalDelta) * 0.08
	if event.ArousalDelta > 0 {
		state.Mood.MoodArousal = clampSignedDelta(state.Mood.MoodArousal, moodArousalTransfer)
	} else if event.ArousalDelta < 0 {
		state.Mood.MoodArousal = clampSignedDelta(state.Mood.MoodArousal, -moodArousalTransfer)
	}

	state.Stress = clampSignedDelta(state.Stress, event.StressDelta)
	state.Energy = clampSignedDelta(state.Energy, event.EnergyDelta)
	state.UpdatedAt = event.Timestamp
	return state
}

func CreateSnapshot(state PsycheState) PsycheSnapshot {
	return PsycheSnapshot{
		ID:               uuid.NewString(),
		CharacterID:      state.CharacterID,
		Version:          state.Version,
		Timestamp:        state.UpdatedAt,
		EmotionValence:   state.Emotion.Valence,
		EmotionArousal:   state.Emotion.Arousal,
		EmotionDominance: state.Emotion.Dominance,
		MoodValence:      state.Mood.MoodValence,
		MoodArousal:      state.Mood.MoodArousal,
		Stress:           state.Stress,
		Energy:           state.Energy,
	}
}

func clampSignedDelta(current, delta float64) float64 {
	result := current + delta
	if result < 0 {
		return 0
	}
	if result > 1 {
		return 1
	}
	return math.Round(result*10000) / 10000
}
