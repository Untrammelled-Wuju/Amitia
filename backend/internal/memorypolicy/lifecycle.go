package memorypolicy

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

type Lifecycle struct {
	RetentionLevel    int     `json:"retentionLevel"`
	MemoryStrength    float64 `json:"memoryStrength"`
	Pinned            bool    `json:"pinned"`
	DecayState        string  `json:"decayState"`
	StrengthUpdatedAt string  `json:"strengthUpdatedAt"`
	LastReinforcedAt  string  `json:"lastReinforcedAt,omitempty"`
	ReinforceCount    int     `json:"reinforceCount"`
}

func NewLifecycle(level int, pinned bool, importance int, now time.Time) Lifecycle {
	if level == 0 {
		level = 5
		if importance >= 9 {
			level = 2
		} else if importance >= 7 {
			level = 3
		} else if importance >= 4 {
			level = 4
		}
	}
	if pinned {
		level = 1
	}
	return Lifecycle{RetentionLevel: level, MemoryStrength: DefaultStrength(level), Pinned: pinned, DecayState: "active", StrengthUpdatedAt: now.UTC().Format(time.RFC3339Nano)}
}

func AssignLifecycle(memoryType, subtype string, importance int, pinned bool, now time.Time) Lifecycle {
	state := NewLifecycle(0, pinned, importance, now)
	if pinned {
		return state
	}
	minimum := 5
	switch strings.ToUpper(strings.TrimSpace(subtype)) {
	case "MOOD", "NOW":
		state.RetentionLevel = 5
	case "PLANS":
		minimum = 4
	case "PROJECTS", "LEARNING", "ROUTINES", "TASTES", "LIFESTYLE", "PROCEDURES":
		minimum = 3
	case "BASIC_PROFILE", "OUR_BOND", "COMMITMENTS", "LIFE_STORY", "VALUES_BELIEFS", "FAMILY", "FRIENDS", "PARTNER", "CAREER", "GOALS":
		minimum = 2
	}
	if minimum < state.RetentionLevel {
		state.RetentionLevel = minimum
	}
	if memoryType == "personal_info" && importance >= 9 {
		state.RetentionLevel = 1
	}
	state.MemoryStrength = DefaultStrength(state.RetentionLevel)
	return state
}

func HalfLife(level int) time.Duration {
	if level < 1 || level > 5 {
		level = 3
	}
	return time.Duration([]int{36500, 365, 90, 30, 7}[level-1]) * 24 * time.Hour
}

func ReinforcedStrength(strength float64, count int) float64 {
	if strength <= 0 {
		strength = .5
	}
	increment := .10 + math.Min(float64(count), 5)*.01
	return math.Max(0, math.Min(1, strength+(1-strength)*increment))
}

func ReinforcedLevel(level int, strength float64, count int) int {
	if level < 1 || level > 5 {
		level = 3
	}
	if level > 1 && count >= 2 && strength >= .8 {
		return level - 1
	}
	return level
}

func DefaultStrength(level int) float64 {
	if level < 1 || level > 5 {
		level = 3
	}
	return []float64{1, .86, .68, .5, .36}[level-1]
}

func (state Lifecycle) Validate() error {
	if state.RetentionLevel < 1 || state.RetentionLevel > 5 || math.IsNaN(state.MemoryStrength) || math.IsInf(state.MemoryStrength, 0) || state.MemoryStrength < 0 || state.MemoryStrength > 1 || state.ReinforceCount < 0 {
		return errors.New("记忆等级或强度无效")
	}
	return nil
}

func (state Lifecycle) Effective(now time.Time) Lifecycle {
	if state.RetentionLevel < 1 || state.RetentionLevel > 5 {
		state.RetentionLevel = 3
	}
	if state.Pinned || state.RetentionLevel == 1 {
		state.MemoryStrength, state.DecayState = 1, "active"
		return state
	}
	anchor, err := time.Parse(time.RFC3339Nano, state.StrengthUpdatedAt)
	if err == nil && now.After(anchor) {
		state.MemoryStrength *= math.Pow(2, -now.Sub(anchor).Hours()/HalfLife(state.RetentionLevel).Hours())
	}
	thresholds := []float64{0, .40, .30, .18, .08}
	for state.RetentionLevel < 5 && state.MemoryStrength < thresholds[state.RetentionLevel-1] {
		state.RetentionLevel++
	}
	state.DecayState = "active"
	if state.RetentionLevel == 5 && state.MemoryStrength < .08 {
		state.DecayState = "archived"
	} else if state.MemoryStrength < thresholds[state.RetentionLevel-1]+.12 {
		state.DecayState = "fading"
	}
	return state
}

func (state Lifecycle) Reinforce(now time.Time) Lifecycle {
	state = state.Effective(now)
	if state.Pinned || state.RetentionLevel == 1 {
		return state
	}
	state.ReinforceCount++
	state.MemoryStrength = ReinforcedStrength(state.MemoryStrength, state.ReinforceCount)
	state.RetentionLevel = ReinforcedLevel(state.RetentionLevel, state.MemoryStrength, state.ReinforceCount)
	state.DecayState = "active"
	state.StrengthUpdatedAt = now.UTC().Format(time.RFC3339Nano)
	state.LastReinforcedAt = state.StrengthUpdatedAt
	return state
}

func ReadLifecycle(document map[string]json.RawMessage, now time.Time) Lifecycle {
	var state Lifecycle
	encoded, _ := json.Marshal(document)
	_ = json.Unmarshal(encoded, &state)
	if state.RetentionLevel == 0 {
		var content struct {
			Importance int `json:"importance"`
		}
		_ = json.Unmarshal(document["content"], &content)
		state = NewLifecycle(0, false, content.Importance, now)
	}
	return state
}

func WriteLifecycle(document map[string]json.RawMessage, state Lifecycle) {
	encoded, _ := json.Marshal(state)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &fields)
	for key, value := range fields {
		document[key] = value
	}
}

func ProjectLifecycle(raw json.RawMessage, now time.Time) json.RawMessage {
	var document map[string]json.RawMessage
	if json.Unmarshal(raw, &document) != nil || document == nil {
		return raw
	}
	state := ReadLifecycle(document, now).Effective(now)
	for key, value := range map[string]any{"effectiveStrength": state.MemoryStrength, "effectiveRetentionLevel": state.RetentionLevel, "effectiveDecayState": state.DecayState} {
		document[key], _ = json.Marshal(value)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return raw
	}
	return encoded
}
