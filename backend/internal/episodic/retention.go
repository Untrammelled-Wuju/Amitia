// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"math"
	"strings"
	"time"
)

const (
	episodicDecayActive   = "active"
	episodicDecayFading   = "fading"
	episodicDecayArchived = "archived"
)

func normalizeEpisodicRetentionLevel(level int) int {
	if level < 1 || level > 5 {
		return 4
	}
	return level
}

func episodicDefaultStrength(level int) float64 {
	switch normalizeEpisodicRetentionLevel(level) {
	case 1:
		return 1.0
	case 2:
		return 0.86
	case 3:
		return 0.68
	case 4:
		return 0.50
	default:
		return 0.36
	}
}

func episodicHalfLife(level int) time.Duration {
	switch normalizeEpisodicRetentionLevel(level) {
	case 1:
		return 100 * 365 * 24 * time.Hour
	case 2:
		return 365 * 24 * time.Hour
	case 3:
		return 90 * 24 * time.Hour
	case 4:
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

func episodicStrengthAt(m EpisodicMemory, now time.Time) float64 {
	strength := m.MemoryStrength
	if strength <= 0 || strength > 1 {
		strength = episodicDefaultStrength(m.RetentionLevel)
	}
	anchorText := ""
	if m.StrengthUpdatedAt != nil {
		anchorText = strings.TrimSpace(*m.StrengthUpdatedAt)
	}
	if anchorText == "" && m.LastReinforcedAt != nil {
		anchorText = strings.TrimSpace(*m.LastReinforcedAt)
	}
	if anchorText == "" {
		anchorText = strings.TrimSpace(m.CreatedAt)
	}
	anchor, ok := parseEpisodicTime(anchorText)
	if !ok || !now.After(anchor) {
		return clampEpisodicStrength(strength)
	}
	halfLife := episodicHalfLife(m.RetentionLevel)
	if halfLife <= 0 {
		return clampEpisodicStrength(strength)
	}
	decayed := strength * math.Pow(2, -now.Sub(anchor).Seconds()/halfLife.Seconds())
	return clampEpisodicStrength(decayed)
}

func episodicDecayTransition(level int, strength float64) (int, string) {
	level = normalizeEpisodicRetentionLevel(level)
	strength = clampEpisodicStrength(strength)
	for {
		switch level {
		case 1:
			return 1, episodicDecayActive
		case 2:
			if strength < 0.40 {
				level = 3
				continue
			}
		case 3:
			if strength < 0.30 {
				level = 4
				continue
			}
		case 4:
			if strength < 0.18 {
				level = 5
				continue
			}
		case 5:
			if strength < 0.08 {
				return 5, episodicDecayArchived
			}
		}
		break
	}
	if level >= 4 || strength < 0.45 {
		return level, episodicDecayFading
	}
	return level, episodicDecayActive
}

func clampEpisodicStrength(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func parseEpisodicTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *service) maintainRetention(m *EpisodicMemory, now time.Time) {
	if s == nil || s.db == nil || m == nil || strings.TrimSpace(m.ID) == "" {
		return
	}
	strength := episodicStrengthAt(*m, now)
	level, state := episodicDecayTransition(m.RetentionLevel, strength)
	currentState := strings.TrimSpace(m.DecayState)
	if currentState == "" {
		currentState = episodicDecayActive
	}
	if currentState == episodicDecayArchived {
		m.MemoryStrength = strength
		m.RetentionLevel = level
		return
	}
	if level == m.RetentionLevel && state == currentState {
		m.MemoryStrength = strength
		m.DecayState = state
		return
	}
	nowText := now.Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"retention_level":     level,
		"memory_strength":     strength,
		"strength_updated_at": nowText,
		"decay_state":         state,
		"updated_at":          nowText,
	}
	if state == episodicDecayArchived {
		updates["archived_at"] = nowText
	} else {
		updates["archived_at"] = nil
	}
	if err := s.db.Table("episodic_memories").Where("id = ?", m.ID).Updates(updates).Error; err != nil {
		return
	}
	m.RetentionLevel = level
	m.MemoryStrength = strength
	m.DecayState = state
	m.StrengthUpdatedAt = &nowText
	m.UpdatedAt = nowText
	if state == episodicDecayArchived {
		m.ArchivedAt = &nowText
		if s.graphSvc != nil {
			_ = s.graphSvc.DeleteNode("episodic:" + m.ID)
		}
	} else {
		m.ArchivedAt = nil
	}
}
