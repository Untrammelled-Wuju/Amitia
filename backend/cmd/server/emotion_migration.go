package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"gorm.io/gorm"
	amitialog "log"
)

func migrateLegacyEmotionHistory(ctx context.Context, db *gorm.DB, engine *business.Engine, spaceID, deviceID, coreID string) error {
	if db == nil || engine == nil || strings.TrimSpace(spaceID) == "" || strings.TrimSpace(deviceID) == "" || strings.TrimSpace(coreID) == "" {
		return nil
	}
	if db.Migrator().HasTable("moods") {
		var rows []struct {
			ID          int64
			CharacterID string
			Mood        string
			MoodValue   string
			Level       int
			CreatedAt   string
		}
		if err := db.WithContext(ctx).Table("moods").Order("created_at ASC,id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			roleID := strings.TrimSpace(row.CharacterID)
			if roleID == "" {
				continue
			}
			valence := float64(row.Level) / 100
			if parsed, err := parseOptionalFloat(row.MoodValue); err == nil && parsed >= 0 && parsed <= 1 {
				valence = parsed
			}
			snapshot := business.EmotionSnapshot{
				ID:              fmt.Sprintf("emotion/legacy-mood/%d", row.ID),
				SpaceID:         spaceID,
				RoleID:          roleID,
				RequestID:       legacyEmotionRequestID("mood", fmt.Sprint(row.ID)),
				Source:          "legacy-mood",
				MoodValence:     valence,
				MoodArousal:     0.5,
				MoodPressure:    math.Abs(valence - 0.5),
				MoodLabel:       firstNonEmptyContinuity(row.Mood, ownedMoodLabel(valence)),
				AffectLabel:     ownedAffectLabel(valence, 0.5, 0),
				RecoveryApplied: true,
				RecoveredMood:   firstNonEmptyContinuity(row.Mood, ownedMoodLabel(valence)),
				RecoveredLevel:  row.Level,
				LegacyKind:      "mood",
				LegacyPayload:   mustJSON(row),
				CreatedAt:       normalizedLegacyEmotionTime(row.CreatedAt),
			}
			if err := engine.ImportEmotionSnapshot(ctx, business.Request{SpaceID: spaceID, DeviceID: deviceID, TargetDeviceID: deviceID, CoreID: coreID, RoleID: roleID, RequestID: snapshot.RequestID}, snapshot); err != nil {
				return fmt.Errorf("migrate legacy mood %d: %w", row.ID, err)
			}
		}
	}
	if db.Migrator().HasTable("messages") && db.Migrator().HasColumn("messages", "mood") {
		var rows []struct {
			ID             string
			ConversationID string
			CharacterID    string
			Mood           string
			CreatedAt      string
		}
		if err := db.WithContext(ctx).Table("messages").Select("id,conversation_id,character_id,mood,created_at").Where("mood IS NOT NULL AND mood <> ''").Order("created_at ASC,id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			roleID := strings.TrimSpace(row.CharacterID)
			if roleID == "" {
				continue
			}
			valence := moodLabelValence(row.Mood)
			snapshot := business.EmotionSnapshot{
				ID:             "emotion/legacy-message/" + legacyEmotionKey(row.ConversationID, row.ID),
				SpaceID:        spaceID,
				RoleID:         roleID,
				ConversationID: row.ConversationID,
				MessageID:      row.ID,
				RequestID:      legacyEmotionRequestID("message", row.ID),
				Source:         "legacy-message-mood",
				MoodValence:    valence,
				MoodArousal:    0.5,
				MoodPressure:   math.Abs(valence - 0.5),
				MoodLabel:      strings.TrimSpace(row.Mood),
				AffectLabel:    ownedAffectLabel(valence, 0.5, 0),
				LegacyKind:     "message-mood",
				LegacyPayload:  mustJSON(row),
				CreatedAt:      normalizedLegacyEmotionTime(row.CreatedAt),
			}
			if err := engine.ImportEmotionSnapshot(ctx, business.Request{SpaceID: spaceID, DeviceID: deviceID, TargetDeviceID: deviceID, CoreID: coreID, RoleID: roleID, RequestID: snapshot.RequestID}, snapshot); err != nil {
				return fmt.Errorf("migrate legacy message mood %s: %w", row.ID, err)
			}
		}
	}
	if db.Migrator().HasTable("emotion_states") {
		var rows []struct {
			SpaceID                 string
			CharacterID             string
			UserAffectJSON          string
			RelationshipEmotionJSON string
			SignalsJSON             string
			BaselineJSON            string
			Version                 int64
			UpdatedAt               string
		}
		if err := db.WithContext(ctx).Table("emotion_states").Order("updated_at ASC,character_id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			roleID := strings.TrimSpace(row.CharacterID)
			if roleID == "" {
				continue
			}
			var affect struct {
				PrimaryEmotion string  `json:"primary_emotion"`
				Intensity      float64 `json:"intensity"`
				Stress         float64 `json:"stress"`
				Confidence     float64 `json:"confidence"`
			}
			if err := json.Unmarshal([]byte(row.UserAffectJSON), &affect); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			payload, _ := json.Marshal(map[string]json.RawMessage{
				"userAffect":          json.RawMessage(defaultJSON(row.UserAffectJSON)),
				"relationshipEmotion": json.RawMessage(defaultJSON(row.RelationshipEmotionJSON)),
				"signals":             json.RawMessage(defaultJSON(row.SignalsJSON)),
				"baseline":            json.RawMessage(defaultJSON(row.BaselineJSON)),
			})
			snapshot := business.EmotionSnapshot{
				ID:            "emotion/legacy-emotion/" + legacyEmotionKey(row.SpaceID, row.CharacterID),
				SpaceID:       spaceID,
				RoleID:        roleID,
				RequestID:     legacyEmotionRequestID("emotion", row.SpaceID+"\x00"+row.CharacterID),
				Source:        "legacy-emotion",
				Confidence:    math.Max(affect.Confidence, math.Max(affect.Intensity, affect.Stress)),
				Stress:        affect.Stress,
				EmotionLabel:  firstNonEmptyContinuity(affect.PrimaryEmotion, "neutral"),
				MoodValence:   0.5,
				MoodArousal:   0.5,
				MoodPressure:  0.5,
				MoodLabel:     "neutral",
				AffectLabel:   ownedAffectLabel(0.5, 0.5, affect.Stress),
				LegacyKind:    "emotion-state",
				LegacyPayload: payload,
				CreatedAt:     normalizedLegacyEmotionTime(row.UpdatedAt),
			}
			if err := engine.ImportEmotionSnapshot(ctx, business.Request{SpaceID: spaceID, DeviceID: deviceID, TargetDeviceID: deviceID, CoreID: coreID, RoleID: roleID, RequestID: snapshot.RequestID}, snapshot); err != nil {
				return fmt.Errorf("migrate legacy emotion state %s: %w", roleID, err)
			}
		}
	}
	if db.Migrator().HasTable("affect_states") {
		var rows []struct {
			CharacterID string
			StateJSON   string
			Version     int64
			UpdatedAt   string
		}
		if err := db.WithContext(ctx).Table("affect_states").Order("updated_at ASC,character_id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			roleID := strings.TrimSpace(row.CharacterID)
			if roleID == "" {
				continue
			}
			var state struct {
				Emotion struct {
					Positive  float64 `json:"positive"`
					Negative  float64 `json:"negative"`
					Arousal   float64 `json:"arousal"`
					Dominance float64 `json:"dominance"`
				} `json:"emotion"`
				Mood struct {
					Valence float64 `json:"valence"`
					Tension float64 `json:"tension"`
				} `json:"mood"`
				Stress float64 `json:"stress"`
			}
			if err := json.Unmarshal([]byte(row.StateJSON), &state); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			valence := clamp01Continuity((state.Emotion.Positive - state.Emotion.Negative + 1) / 2)
			snapshot := business.EmotionSnapshot{
				ID:            "emotion/legacy-affect/" + legacyEmotionKey("", roleID),
				SpaceID:       spaceID,
				RoleID:        roleID,
				RequestID:     legacyEmotionRequestID("affect", roleID),
				Source:        "legacy-affect",
				Valence:       valence,
				Arousal:       clamp01Continuity(state.Emotion.Arousal),
				Dominance:     clamp01Continuity(state.Emotion.Dominance),
				Stress:        clamp01Continuity(state.Stress),
				MoodValence:   clamp01Continuity(state.Mood.Valence),
				MoodArousal:   clamp01Continuity(state.Mood.Tension),
				EmotionLabel:  ownedEmotionLabel(valence, state.Emotion.Arousal, state.Stress),
				MoodLabel:     ownedMoodLabel(state.Mood.Valence),
				AffectLabel:   ownedAffectLabel(valence, state.Emotion.Arousal, state.Stress),
				LegacyKind:    "affect-state",
				LegacyPayload: json.RawMessage(defaultJSON(row.StateJSON)),
				CreatedAt:     normalizedLegacyEmotionTime(row.UpdatedAt),
			}
			if err := engine.ImportEmotionSnapshot(ctx, business.Request{SpaceID: spaceID, DeviceID: deviceID, TargetDeviceID: deviceID, CoreID: coreID, RoleID: roleID, RequestID: snapshot.RequestID}, snapshot); err != nil {
				return fmt.Errorf("migrate legacy affect state %s: %w", roleID, err)
			}
		}
	}
	return nil
}

func legacyEmotionRequestID(kind, value string) string {
	digest := sha256.Sum256([]byte(kind + "\x00" + value))
	return "emotion-migration/" + kind + "/" + hex.EncodeToString(digest[:16])
}

func legacyEmotionKey(spaceID, roleID string) string {
	digest := sha256.Sum256([]byte(spaceID + "\x00" + roleID))
	return hex.EncodeToString(digest[:16])
}

func normalizedLegacyEmotionTime(value string) string {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func parseOptionalFloat(value string) (float64, error) {
	var parsed float64
	_, err := fmt.Sscan(strings.TrimSpace(value), &parsed)
	return parsed, err
}

func defaultJSON(value string) string {
	if json.Valid([]byte(value)) {
		return value
	}
	return "{}"
}

func clamp01Continuity(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func moodLabelValence(label string) float64 {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "happy":
		return 0.85
	case "calm":
		return 0.65
	case "low":
		return 0.35
	case "sad":
		return 0.15
	default:
		return 0.5
	}
}

func mustJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
