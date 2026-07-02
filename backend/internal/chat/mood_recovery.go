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

func (s *service) moodRecoveryCheck(ctx context.Context, convID, charID, source string) {
	if ctx.Err() != nil {
		return
	}
	if source == "proactive" || source == "system" {
		return
	}
	var lastAt string
	err := s.db.WithContext(ctx).Table("messages").Select("created_at").Where("role = 'user' AND conversation_id = ?", convID).Order("created_at DESC").Offset(1).Limit(1).Row().Scan(&lastAt)
	if err != nil || lastAt == "" {
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", lastAt, time.Local)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", lastAt)
	}
	if err != nil {
		return
	}
	idleDur := time.Since(t)
	if idleDur > 6*time.Hour {
		if ctx.Err() != nil {
			return
		}
		state, ok := s.recoveredMoodState(ctx, charID, idleDur)
		if !ok {
			return
		}
		s.db.WithContext(ctx).Exec("INSERT INTO moods (character_id, mood, level, created_at) VALUES (?, ?, ?, datetime('now', 'localtime'))", charID, state.Mood, state.Level)
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
	if idleDur <= 6*time.Hour {
		return value
	}
	hours := idleDur.Hours() - 6
	ratio := math.Min(1, hours/18)
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
