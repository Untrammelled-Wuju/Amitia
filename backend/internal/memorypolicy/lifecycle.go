package memorypolicy

import (
	"encoding/json"
	"errors"
	amitialog "log"
	"math"
	"strings"
	"time"
)

type Lifecycle struct {
	RetentionLevel    int     `json:"retentionLevel"`
	MemoryStrength    float64 `json:"memoryStrength"`
	Pinned            bool    `json:"pinned"`
	DecayState        string  `json:"decayState"`
	DecayPolicy       string  `json:"decayPolicy,omitempty"`
	StrengthUpdatedAt string  `json:"strengthUpdatedAt"`
	LastReinforcedAt  string  `json:"lastReinforcedAt,omitempty"`
	ReinforceCount    int     `json:"reinforceCount"`
	StabilityDays     float64 `json:"stabilityDays,omitempty"`
	Difficulty        float64 `json:"difficulty,omitempty"`
	RecallProbability float64 `json:"recallProbability,omitempty"`
	ReviewCount       int     `json:"reviewCount,omitempty"`
	LapseCount        int     `json:"lapseCount,omitempty"`
	NextReviewAt      string  `json:"nextReviewAt,omitempty"`
}

const (
	DecayPolicyDurable  = "durable"
	DecayPolicyEpisodic = "episodic"
	DecayPolicyAdaptive = "adaptive"
)

func NewLifecycle(level int, pinned bool, importance int, now time.Time) Lifecycle {
	return NewLifecycleWithPolicy(DecayPolicyAdaptive, level, pinned, importance, now)
}

func NewLifecycleWithPolicy(policy string, level int, pinned bool, importance int, now time.Time) Lifecycle {
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
	return Lifecycle{RetentionLevel: level, MemoryStrength: DefaultStrength(level), Pinned: pinned, DecayState: "active", DecayPolicy: normalizePolicy(policy), StabilityDays: DefaultStabilityDays(level), Difficulty: 0.5, RecallProbability: DefaultStrength(level), StrengthUpdatedAt: now.UTC().Format(time.RFC3339Nano)}
}

func AssignLifecycle(memoryType, subtype string, importance int, pinned bool, now time.Time) Lifecycle {
	return AssignLifecycleForKind("", memoryType, subtype, importance, pinned, now)
}

func AssignLifecycleForKind(kind, memoryType, subtype string, importance int, pinned bool, now time.Time) Lifecycle {
	policy := DecayPolicyFor(kind, memoryType, subtype)
	state := NewLifecycleWithPolicy(policy, 0, pinned, importance, now)
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

func DefaultStabilityDays(level int) float64 {
	return []float64{36500, 365, 90, 30, 7}[level-1]
}

func (state Lifecycle) Validate() error {
	if state.RetentionLevel < 1 || state.RetentionLevel > 5 || math.IsNaN(state.MemoryStrength) || math.IsInf(state.MemoryStrength, 0) || state.MemoryStrength < 0 || state.MemoryStrength > 1 || state.ReinforceCount < 0 || state.StabilityDays < 0 || state.Difficulty < 0 || state.Difficulty > 1 || state.RecallProbability < 0 || state.RecallProbability > 1 || state.ReviewCount < 0 || state.LapseCount < 0 || !validPolicy(state.DecayPolicy) {
		return errors.New("记忆等级或强度无效")
	}
	return nil
}

func (state Lifecycle) Effective(now time.Time) Lifecycle {
	if state.RetentionLevel < 1 || state.RetentionLevel > 5 {
		state.RetentionLevel = 3
	}
	if state.Pinned || state.RetentionLevel == 1 {
		state.MemoryStrength, state.DecayState, state.RecallProbability = 1, "active", 1
		return state
	}
	halfLife := HalfLife(state.RetentionLevel)
	if state.StabilityDays > 0 {
		halfLife = time.Duration(state.StabilityDays * float64(24*time.Hour))
	}
	anchor, err := time.Parse(time.RFC3339Nano, state.StrengthUpdatedAt)
	if err == nil && now.After(anchor) {
		state.MemoryStrength *= math.Pow(2, -now.Sub(anchor).Hours()/halfLife.Hours())
	}
	state.RecallProbability = state.MemoryStrength
	thresholds := []float64{0, .40, .30, .18, .08}
	for state.RetentionLevel < 5 && state.MemoryStrength < thresholds[state.RetentionLevel-1] {
		state.RetentionLevel++
	}
	state.DecayState = "active"
	if state.RetentionLevel == 5 && state.MemoryStrength < .08 {
		if state.DecayPolicy == DecayPolicyDurable {
			state.DecayState = "dormant"
		} else {
			state.DecayState = "archived"
		}
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
	state.RecallProbability = state.MemoryStrength
	state.Difficulty = math.Max(0, state.Difficulty-.03)
	state.StabilityDays = math.Min(1095, math.Max(1, state.StabilityDays*(1.15+math.Min(float64(state.ReinforceCount), 5)*.02)))
	state.RetentionLevel = ReinforcedLevel(state.RetentionLevel, state.MemoryStrength, state.ReinforceCount)
	state.DecayState = "active"
	state.StrengthUpdatedAt = now.UTC().Format(time.RFC3339Nano)
	state.LastReinforcedAt = state.StrengthUpdatedAt
	state.ReviewCount++
	state.NextReviewAt = now.Add(time.Duration(state.StabilityDays * float64(24*time.Hour))).UTC().Format(time.RFC3339Nano)
	return state
}

func ReadLifecycle(document map[string]json.RawMessage, now time.Time) Lifecycle {
	var state Lifecycle
	encoded, err := json.Marshal(document)
	if err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if err := json.Unmarshal(encoded, &state); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if state.DecayPolicy == "" {
		state.DecayPolicy = DecayPolicyForDocument(document)
	}
	if state.RetentionLevel == 0 {
		var content struct {
			Importance int `json:"importance"`
		}
		if err := json.Unmarshal(document["content"], &content); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		state = NewLifecycleWithPolicy(state.DecayPolicy, 0, false, content.Importance, now)
	}
	return state
}

func WriteLifecycle(document map[string]json.RawMessage, state Lifecycle) {
	encoded, err := json.Marshal(state)
	if err != nil {
		amitialog.Printf("[original-error] %v", err)
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
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
	for key, value := range map[string]any{"effectiveStrength": state.MemoryStrength, "effectiveRetentionLevel": state.RetentionLevel, "effectiveDecayState": state.DecayState, "effectiveDecayPolicy": state.DecayPolicy, "effectiveStabilityDays": state.StabilityDays, "effectiveDifficulty": state.Difficulty, "effectiveRecallProbability": state.RecallProbability} {
		encoded, err := json.Marshal(value)
		if err != nil {
			amitialog.Printf("[original-error] %v", err)
			return raw
		}
		document[key] = encoded
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return raw
	}
	return encoded
}

func DecayPolicyFor(kind, memoryType, subtype string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	memoryType = strings.ToLower(strings.TrimSpace(memoryType))
	subtype = strings.ToUpper(strings.TrimSpace(subtype))
	if kind == "episodic" || memoryType == "episodic" {
		return DecayPolicyEpisodic
	}
	switch subtype {
	case "MOOD", "NOW", "PLANS":
		return DecayPolicyEpisodic
	}
	switch kind {
	case "memory", "fact", "profile":
		return DecayPolicyDurable
	}
	switch memoryType {
	case "personal_info", "hobby", "preference", "fact", "habit", "relationship", "profile":
		return DecayPolicyDurable
	}
	return DecayPolicyAdaptive
}

func DecayPolicyForDocument(document map[string]json.RawMessage) string {
	read := func(value map[string]json.RawMessage, key string) string {
		raw, exists := value[key]
		if !exists || len(raw) == 0 {
			return ""
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return text
	}
	kind := read(document, "kind")
	var content map[string]json.RawMessage
	if raw, exists := document["content"]; exists && len(raw) > 0 {
		if err := json.Unmarshal(raw, &content); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	return DecayPolicyFor(kind, read(content, "memoryType"), read(content, "memorySubtype"))
}

func normalizePolicy(policy string) string {
	policy = strings.ToLower(strings.TrimSpace(policy))
	if validPolicy(policy) {
		return policy
	}
	return DecayPolicyAdaptive
}

func validPolicy(policy string) bool {
	switch policy {
	case DecayPolicyDurable, DecayPolicyEpisodic, DecayPolicyAdaptive:
		return true
	default:
		return false
	}
}
