// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"math"
	"strings"
	"time"
)

const (
	RetentionL1 = 1
	RetentionL2 = 2
	RetentionL3 = 3
	RetentionL4 = 4
	RetentionL5 = 5
)

const (
	DecayStateActive   = "active"
	DecayStateFading   = "fading"
	DecayStateArchived = "archived"
)

type RetentionAssignment struct {
	Level    int
	Strength float64
	Pinned   bool
}

func AssignRetention(memoryType, subtype string, importance int, explicitPinned bool) RetentionAssignment {
	if explicitPinned {
		return RetentionAssignment{Level: RetentionL1, Strength: 1, Pinned: true}
	}

	level := RetentionL5
	switch {
	case importance >= 9:
		level = RetentionL2
	case importance >= 7:
		level = RetentionL3
	case importance >= 4:
		level = RetentionL4
	}

	subtype = strings.ToUpper(strings.TrimSpace(subtype))
	switch subtype {
	case "MOOD", "NOW":
		level = RetentionL5
	case "PLANS":
		level = minRetentionLevel(level, RetentionL4)
	case "PROJECTS", "LEARNING", "ROUTINES", "TASTES", "LIFESTYLE", "PROCEDURES":
		level = minRetentionLevel(level, RetentionL3)
	case "BASIC_PROFILE", "OUR_BOND", "COMMITMENTS", "LIFE_STORY", "VALUES_BELIEFS", "FAMILY", "FRIENDS", "PARTNER", "CAREER", "GOALS":
		level = minRetentionLevel(level, RetentionL2)
	}

	mt := CanonicalMemoryType(memoryType)
	if mt == MemoryTypePersonalInfo && importance >= 9 {
		level = RetentionL1
	}

	strength := defaultStrengthForLevel(level)
	return RetentionAssignment{Level: level, Strength: strength}
}

func minRetentionLevel(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

func normalizeRetentionLevel(level int) int {
	if level < RetentionL1 || level > RetentionL5 {
		return RetentionL3
	}
	return level
}

func defaultStrengthForLevel(level int) float64 {
	switch normalizeRetentionLevel(level) {
	case RetentionL1:
		return 1.0
	case RetentionL2:
		return 0.86
	case RetentionL3:
		return 0.68
	case RetentionL4:
		return 0.5
	default:
		return 0.36
	}
}

func retentionHalfLife(level int) time.Duration {
	switch normalizeRetentionLevel(level) {
	case RetentionL1:
		return 100 * 365 * 24 * time.Hour
	case RetentionL2:
		return 365 * 24 * time.Hour
	case RetentionL3:
		return 90 * 24 * time.Hour
	case RetentionL4:
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

func memoryEffectiveStrength(m Memory, now time.Time) float64 {
	if m.Pinned || normalizeRetentionLevel(m.RetentionLevel) == RetentionL1 {
		return 1
	}
	base := m.MemoryStrength
	if base <= 0 {
		base = defaultStrengthForLevel(m.RetentionLevel)
	}
	anchor := parseMemoryTime(m.CreatedAt)
	if m.StrengthUpdatedAt != nil && strings.TrimSpace(*m.StrengthUpdatedAt) != "" {
		if t := parseMemoryTime(*m.StrengthUpdatedAt); !t.IsZero() {
			anchor = t
		}
	} else if m.LastReinforcedAt != nil && strings.TrimSpace(*m.LastReinforcedAt) != "" {
		if t := parseMemoryTime(*m.LastReinforcedAt); !t.IsZero() {
			anchor = t
		}
	}
	if anchor.IsZero() || now.Before(anchor) {
		return clamp01(base)
	}
	halfLife := retentionHalfLife(m.RetentionLevel)
	decay := math.Pow(2, -now.Sub(anchor).Hours()/halfLife.Hours())
	return clamp01(base * decay)
}

func retentionFactor(m Memory, explicitRecall bool, now time.Time) float64 {
	strength := memoryEffectiveStrength(m, now)
	factor := 0.85 + strength*0.20
	if explicitRecall && strings.EqualFold(m.DecayState, DecayStateArchived) {
		factor *= 0.92
	}
	return factor
}

func reinforcementStrength(current float64, reinforceCount int) float64 {
	if current <= 0 {
		current = 0.5
	}
	increment := 0.10 + math.Min(float64(reinforceCount), 5)*0.01
	return clamp01(current + (1-current)*increment)
}

func suggestedRetentionAfterReinforce(level int, strength float64, reinforceCount int) int {
	level = normalizeRetentionLevel(level)
	if level <= RetentionL1 {
		return RetentionL1
	}
	if reinforceCount >= 2 && strength >= 0.8 {
		return level - 1
	}
	return level
}

func decayStateFor(level int, strength float64) (string, int) {
	level = normalizeRetentionLevel(level)
	if level == RetentionL1 {
		return DecayStateActive, RetentionL1
	}
	thresholds := map[int]float64{
		RetentionL2: 0.40,
		RetentionL3: 0.30,
		RetentionL4: 0.18,
		RetentionL5: 0.08,
	}
	for level < RetentionL5 && strength < thresholds[level] {
		level++
	}
	threshold := thresholds[level]
	if level == RetentionL5 && strength < threshold {
		return DecayStateArchived, RetentionL5
	}
	if strength < threshold+0.12 {
		return DecayStateFading, level
	}
	return DecayStateActive, level
}

func parseMemoryTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
