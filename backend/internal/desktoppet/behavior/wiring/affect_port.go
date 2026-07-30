package wiring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/psyche"
)

type AffectAdapter struct {
	store psyche.PsycheStore
}

func NewAffectAdapter(store psyche.PsycheStore) *AffectAdapter {
	return &AffectAdapter{store: store}
}

func (a *AffectAdapter) GetAffectSnapshot(ctx context.Context, userID, characterID string) (*behavior.AffectBehaviorSnapshot, error) {
	state, err := a.store.LoadState(characterID)
	if err != nil {
		if errors.Is(err, psyche.ErrStateNotFound) || strings.Contains(err.Error(), "state not found") {
			return &behavior.AffectBehaviorSnapshot{
				Version:    "v0",
				Label:      "neutral",
				Confidence: 0.5,
				UpdatedAt:  time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("affect adapter: load state failed: %w", err)
	}

	return &behavior.AffectBehaviorSnapshot{
		Version:    state.Version,
		Valence:    state.Emotion.Valence,
		Arousal:    state.Emotion.Arousal,
		Tension:    1.0 - state.Emotion.Dominance,
		Stress:     state.Stress,
		Label:      mapEmotionLabel(state.Emotion.Valence, state.Emotion.Arousal),
		Confidence: computeConfidence(state),
		UpdatedAt:  state.UpdatedAt,
	}, nil
}

func mapEmotionLabel(valence, arousal float64) string {
	if valence > 0.3 && arousal > 0.3 {
		return "happy"
	}
	if valence > 0.3 && arousal <= 0.3 {
		return "calm"
	}
	if valence <= -0.3 && arousal > 0.3 {
		return "angry"
	}
	if valence <= -0.3 && arousal <= 0.3 {
		return "sad"
	}
	if arousal > 0.5 {
		return "excited"
	}
	return "neutral"
}

func computeConfidence(state *psyche.PsycheState) float64 {
	confidence := 0.7
	if state.StateVersion > 0 {
		confidence = 0.9
	}
	if state.Stress > 0.7 {
		confidence -= 0.1
	}
	if confidence < 0.3 {
		confidence = 0.3
	}
	return confidence
}

var _ behavior.CharacterAffectPort = (*AffectAdapter)(nil)
