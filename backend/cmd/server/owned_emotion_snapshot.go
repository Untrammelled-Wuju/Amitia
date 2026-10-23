package main

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

func ownedMoodDetectionSettings(ctx context.Context, db *gorm.DB) (bool, float64, error) {
	enabled, threshold := false, 0.5
	if db == nil {
		return enabled, threshold, nil
	}
	var rows []struct {
		Key   string
		Value string
	}
	if err := db.WithContext(ctx).Table("app_settings").Select("key,value").Where("key IN ? AND deleted_at IS NULL", []string{"mood_detection_enabled", "mood_detection_threshold"}).Find(&rows).Error; err != nil {
		return false, threshold, err
	}
	for _, row := range rows {
		switch row.Key {
		case "mood_detection_enabled":
			enabled = strings.EqualFold(strings.TrimSpace(row.Value), "true")
		case "mood_detection_threshold":
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(row.Value), 64); err == nil && parsed >= 0 && parsed <= 1 {
				threshold = parsed
			}
		}
	}
	return enabled, threshold, nil
}

func buildOwnedEmotionSnapshot(scope coordination.ExecutionScope, request coordination.OwnedChatEffectsRequest, previous, current time.Time, state interaction.PsycheState, threshold float64) business.EmotionSnapshot {
	confidence := math.Max(math.Abs(state.Valence-0.5)*2, math.Abs(state.Arousal-0.5)*2)
	confidence = math.Max(confidence, math.Abs(state.MoodValence-0.5)*2)
	confidence = math.Max(confidence, math.Max(state.Stress, state.MoodPressure))
	confidence = math.Max(0, math.Min(1, confidence))
	recoveryApplied := false
	recoveredMood := ""
	recoveredLevel := 0
	if !previous.IsZero() {
		idle := current.Sub(previous)
		if idle > 6*time.Hour {
			hours := idle.Hours() - 6
			ratio := math.Min(1, hours/18)
			recovered := state.MoodValence + (0.5-state.MoodValence)*ratio
			recoveryApplied = true
			recoveredMood = ownedMoodLabel(recovered)
			recoveredLevel = int(math.Round(math.Max(0, math.Min(1, recovered)) * 100))
		}
	}
	return business.EmotionSnapshot{
		ID:                 "emotion/" + request.RequestID,
		SpaceID:            scope.SpaceID,
		RoleID:             scope.RoleID,
		ConversationID:     request.ConversationID,
		MessageID:          request.MessageID,
		RequestID:          request.RequestID,
		Source:             request.Source,
		DetectionEnabled:   true,
		DetectionThreshold: threshold,
		Confidence:         confidence,
		Valence:            state.Valence,
		Arousal:            state.Arousal,
		Dominance:          state.Dominance,
		Stress:             state.Stress,
		Fatigue:            state.Fatigue,
		MoodValence:        state.MoodValence,
		MoodArousal:        state.MoodArousal,
		MoodPressure:       state.MoodPressure,
		EmotionLabel:       ownedEmotionLabel(state.Valence, state.Arousal, state.Stress),
		MoodLabel:          ownedMoodLabel(state.MoodValence),
		AffectLabel:        ownedAffectLabel(state.Valence, state.Arousal, state.Stress),
		RecoveryApplied:    recoveryApplied,
		RecoveredMood:      recoveredMood,
		RecoveredLevel:     recoveredLevel,
		CreatedAt:          current.UTC().Format(time.RFC3339Nano),
	}
}

func ownedEmotionSnapshotBody(snapshot business.EmotionSnapshot) json.RawMessage {
	raw, _ := json.Marshal(snapshot)
	return raw
}

func ownedEmotionLabel(valence, arousal, stress float64) string {
	if stress > 0.65 {
		return "tense"
	}
	if valence > 0.6 && arousal > 0.55 {
		return "excited"
	}
	if valence > 0.55 {
		return "positive"
	}
	if valence < 0.35 && arousal > 0.55 {
		return "angry"
	}
	if valence < 0.4 {
		return "low"
	}
	return "neutral"
}

func ownedMoodLabel(valence float64) string {
	switch {
	case valence >= 0.7:
		return "happy"
	case valence >= 0.55:
		return "calm"
	case valence >= 0.45:
		return "neutral"
	case valence >= 0.3:
		return "low"
	default:
		return "sad"
	}
}

func ownedAffectLabel(valence, arousal, stress float64) string {
	if stress > 0.55 {
		return "紧张"
	}
	if valence > 0.55 {
		return "积极"
	}
	if valence < 0.45 {
		return "消极"
	}
	if arousal > 0.65 {
		return "活跃"
	}
	return "平静"
}
