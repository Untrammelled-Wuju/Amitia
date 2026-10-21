package chat

import (
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
	moodRecoveryProactive     = "proactive"
	moodRecoverySystem        = "system"
)

func recoveredMoodStateFromValue(current float64, idleDur time.Duration) (moodRecoveryState, bool) {
	valence := recoverMoodValue(current, idleDur)
	return moodRecoveryState{Mood: moodLabelFromValence(valence), Level: clampMoodLevel(int(math.Round(valence * 100)))}, true
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
