// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"math"
	"time"
)

type moodRecoveryState struct {
	Mood  string
	Level int
}

type moodRecoveryRecord struct {
	CharacterID string `gorm:"column:character_id"`
	Mood        string `gorm:"column:mood"`
	Level       int    `gorm:"column:level"`
	CreatedAt   string `gorm:"column:created_at"`
}

func (moodRecoveryRecord) TableName() string { return "moods" }

const (
	moodRecoveryIdleThreshold = 6 * time.Hour
	moodRecoveryFullDuration  = 18 * time.Hour
	moodRecoveryTimeLayout    = "2006-01-02 15:04:05"
	moodRecoveryISOLayout     = "2006-01-02T15:04:05Z"
	moodRecoveryProactive     = "proactive"
	moodRecoverySystem        = "system"
)

func (s *service) moodRecoveryCheck(ctx context.Context, convID, charID, source string) {
	if ctx.Err() != nil {
		return
	}
	if source == moodRecoveryProactive || source == moodRecoverySystem {
		return
	}
	var lastAt string
	err := s.db.WithContext(ctx).Table("messages").Select("created_at").Where("role = 'user' AND conversation_id = ?", convID).Order("created_at DESC").Offset(1).Limit(1).Row().Scan(&lastAt)
	if err != nil || lastAt == "" {
		return
	}
	t, err := time.ParseInLocation(moodRecoveryTimeLayout, lastAt, time.Local)
	if err != nil {
		t, err = time.Parse(moodRecoveryISOLayout, lastAt)
	}
	if err != nil {
		return
	}
	idleDur := time.Since(t)
	if idleDur > moodRecoveryIdleThreshold {
		if ctx.Err() != nil {
			return
		}
		state, ok := s.recoveredMoodState(ctx, charID, idleDur)
		if !ok {
			return
		}
		s.db.WithContext(ctx).Create(&moodRecoveryRecord{
			CharacterID: charID,
			Mood:        state.Mood,
			Level:       state.Level,
			CreatedAt:   time.Now().Format(moodRecoveryTimeLayout),
		})
	}
}

func (s *service) recoveredMoodState(ctx context.Context, charID string, idleDur time.Duration) (moodRecoveryState, bool) {
	if charID == "" {
		return moodRecoveryState{}, false
	}
	var current moodRecoveryState
	err := s.db.WithContext(ctx).Table("moods").Select("mood, level").Where("character_id = ? AND mood != ''", charID).Order("created_at DESC").Limit(1).Row().Scan(&current.Mood, &current.Level)
	if err == nil && current.Mood != "" {
		current.Level = recoverMoodLevel(current.Level, idleDur)
		current.Mood = moodLabelFromValence(float64(current.Level) / 100)
		return current, true
	}
	if s.psycheStore == nil {
		return moodRecoveryState{}, false
	}
	state, err := s.psycheStore.LoadState(charID)
	if err != nil {
		return moodRecoveryState{}, false
	}
	valence := recoverMoodValue(state.Mood.MoodValence, idleDur)
	return moodRecoveryState{Mood: moodLabelFromValence(valence), Level: clampMoodLevel(int(math.Round(valence * 100)))}, true
}

func recoverMoodLevel(level int, idleDur time.Duration) int {
	value := float64(clampMoodLevel(level)) / 100
	return clampMoodLevel(int(math.Round(recoverMoodValue(value, idleDur) * 100)))
}

func recoverMoodValue(value float64, idleDur time.Duration) float64 {
	if idleDur <= moodRecoveryIdleThreshold {
		return value
	}
	hours := idleDur.Hours() - moodRecoveryIdleThreshold.Hours()
	ratio := math.Min(1, hours/moodRecoveryFullDuration.Hours())
	return value + (0.5-value)*ratio
}

func moodLabelFromValence(valence float64) string {
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

func clampMoodLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level > 100 {
		return 100
	}
	return level
}
