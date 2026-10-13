// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"github.com/u-ai/backend/internal/memorypolicy"
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
	state := memorypolicy.AssignLifecycle(string(CanonicalMemoryType(memoryType)), subtype, importance, explicitPinned, time.Now())
	return RetentionAssignment{Level: state.RetentionLevel, Strength: state.MemoryStrength, Pinned: state.Pinned}

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
	return memorypolicy.DefaultStrength(level)

}

func retentionHalfLife(level int) time.Duration {
	return memorypolicy.HalfLife(level)

}

func memoryEffectiveStrength(m Memory, now time.Time) float64 {
	anchor := parseMemoryTime(m.CreatedAt)
	if m.StrengthUpdatedAt != nil && strings.TrimSpace(*m.StrengthUpdatedAt) != "" {
		anchor = parseMemoryTime(*m.StrengthUpdatedAt)
	} else if m.LastReinforcedAt != nil {
		anchor = parseMemoryTime(*m.LastReinforcedAt)
	}
	strength := m.MemoryStrength
	if strength <= 0 {
		strength = defaultStrengthForLevel(m.RetentionLevel)
	}
	state := memorypolicy.Lifecycle{RetentionLevel: normalizeRetentionLevel(m.RetentionLevel), MemoryStrength: strength, Pinned: m.Pinned}
	if !anchor.IsZero() {
		state.StrengthUpdatedAt = anchor.UTC().Format(time.RFC3339Nano)
	}
	return state.Effective(now).MemoryStrength

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
	return memorypolicy.ReinforcedStrength(current, reinforceCount)

}

func suggestedRetentionAfterReinforce(level int, strength float64, reinforceCount int) int {
	return memorypolicy.ReinforcedLevel(level, strength, reinforceCount)

}

func decayStateFor(level int, strength float64) (string, int) {
	state := (memorypolicy.Lifecycle{RetentionLevel: normalizeRetentionLevel(level), MemoryStrength: strength}).Effective(time.Now())
	return state.DecayState, state.RetentionLevel

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
