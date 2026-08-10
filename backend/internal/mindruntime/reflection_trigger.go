package mindruntime

import (
	"time"
)

type ReflectionTriggerKind string

const (
	TriggerKindTime           ReflectionTriggerKind = "time"
	TriggerKindEventCount     ReflectionTriggerKind = "event_count"
	TriggerKindRelationChange ReflectionTriggerKind = "relation_change"
	TriggerKindAnomaly        ReflectionTriggerKind = "anomaly"
)

type ReflectionTriggerConfig struct {
	TimeThreshold           time.Duration
	EventCountThreshold     int
	RelationChangeThreshold int
	AnomalyScoreThreshold   float64
}

func DefaultReflectionTriggerConfig() ReflectionTriggerConfig {
	return ReflectionTriggerConfig{
		TimeThreshold:           24 * time.Hour,
		EventCountThreshold:     50,
		RelationChangeThreshold: 3,
		AnomalyScoreThreshold:   0.7,
	}
}

type ReflectionTriggerState struct {
	LastReflectionAt    time.Time
	EventCountSinceLast int
	RelationChangeCount int
	MaxAnomalyScore     float64
}

type TriggerResult struct {
	Fired bool
	Kinds []ReflectionTriggerKind
	State ReflectionTriggerState
}

func EvaluateTrigger(state ReflectionTriggerState, config ReflectionTriggerConfig, now time.Time, eventCount int, relationChanges int, anomalyScores []float64) TriggerResult {
	result := TriggerResult{
		State: newTriggerState(state, eventCount, relationChanges, anomalyScores),
	}

	firedKinds := make([]ReflectionTriggerKind, 0)

	if config.TimeThreshold > 0 && !state.LastReflectionAt.IsZero() {
		if now.Sub(state.LastReflectionAt) >= config.TimeThreshold {
			firedKinds = append(firedKinds, TriggerKindTime)
		}
	}

	if config.EventCountThreshold > 0 {
		total := state.EventCountSinceLast + eventCount
		if total >= config.EventCountThreshold {
			firedKinds = append(firedKinds, TriggerKindEventCount)
		}
	}

	if config.RelationChangeThreshold > 0 {
		total := state.RelationChangeCount + relationChanges
		if total >= config.RelationChangeThreshold {
			firedKinds = append(firedKinds, TriggerKindRelationChange)
		}
	}

	if config.AnomalyScoreThreshold > 0 {
		bumped := false
		for _, s := range anomalyScores {
			if s >= config.AnomalyScoreThreshold {
				bumped = true
				break
			}
		}
		if state.MaxAnomalyScore >= config.AnomalyScoreThreshold {
			bumped = true
		}
		if bumped {
			firedKinds = append(firedKinds, TriggerKindAnomaly)
		}
	}

	if len(firedKinds) > 0 {
		result.Fired = true
		result.Kinds = firedKinds
	}

	return result
}

func ResetTriggerState() ReflectionTriggerState {
	return ResetTriggerStateAt(time.Now().UTC())
}

func ResetTriggerStateAt(now time.Time) ReflectionTriggerState {
	return ReflectionTriggerState{
		LastReflectionAt:    now.UTC(),
		EventCountSinceLast: 0,
		RelationChangeCount: 0,
		MaxAnomalyScore:     0,
	}
}

func newTriggerState(prev ReflectionTriggerState, eventCount int, relationChanges int, anomalyScores []float64) ReflectionTriggerState {
	st := ReflectionTriggerState{
		LastReflectionAt:    prev.LastReflectionAt,
		EventCountSinceLast: prev.EventCountSinceLast + eventCount,
		RelationChangeCount: prev.RelationChangeCount + relationChanges,
		MaxAnomalyScore:     prev.MaxAnomalyScore,
	}
	for _, s := range anomalyScores {
		if s > st.MaxAnomalyScore {
			st.MaxAnomalyScore = s
		}
	}
	return st
}
