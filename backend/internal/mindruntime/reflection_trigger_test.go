package mindruntime

import (
	"testing"
	"time"
)

func TestEvaluateTrigger_TimeTrigger(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 1 * time.Hour
	config.EventCountThreshold = 0
	config.RelationChangeThreshold = 0
	config.AnomalyScoreThreshold = 0
	pastTime := time.Now().Add(-2 * time.Hour)
	state := ReflectionTriggerState{
		LastReflectionAt: pastTime,
	}
	result := EvaluateTrigger(state, config, time.Now(), 0, 0, nil)
	if !result.Fired {
		t.Error("expected time trigger to fire")
	}
	if len(result.Kinds) != 1 || result.Kinds[0] != TriggerKindTime {
		t.Errorf("expected only time trigger, got %v", result.Kinds)
	}
}

func TestEvaluateTrigger_TimeNotYet(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 1 * time.Hour
	config.EventCountThreshold = 0
	config.RelationChangeThreshold = 0
	config.AnomalyScoreThreshold = 0
	recentTime := time.Now().Add(-30 * time.Minute)
	state := ReflectionTriggerState{
		LastReflectionAt: recentTime,
	}
	result := EvaluateTrigger(state, config, time.Now(), 0, 0, nil)
	if result.Fired {
		t.Error("expected time trigger not to fire")
	}
}

func TestEvaluateTrigger_EventCount(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 0
	config.EventCountThreshold = 5
	config.RelationChangeThreshold = 0
	config.AnomalyScoreThreshold = 0
	state := ReflectionTriggerState{
		EventCountSinceLast: 3,
	}
	result := EvaluateTrigger(state, config, time.Now(), 3, 0, nil)
	if !result.Fired {
		t.Error("expected event count trigger to fire")
	}
}

func TestEvaluateTrigger_RelationChange(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 0
	config.EventCountThreshold = 0
	config.RelationChangeThreshold = 2
	config.AnomalyScoreThreshold = 0
	state := ReflectionTriggerState{
		RelationChangeCount: 1,
	}
	result := EvaluateTrigger(state, config, time.Now(), 0, 2, nil)
	if !result.Fired {
		t.Error("expected relation change trigger to fire")
	}
}

func TestEvaluateTrigger_Anomaly(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 0
	config.EventCountThreshold = 0
	config.RelationChangeThreshold = 0
	config.AnomalyScoreThreshold = 0.7
	state := ReflectionTriggerState{}
	result := EvaluateTrigger(state, config, time.Now(), 0, 0, []float64{0.8, 0.5})
	if !result.Fired {
		t.Error("expected anomaly trigger to fire on 0.8 score")
	}
}

func TestEvaluateTrigger_MultiTrigger(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 1 * time.Hour
	config.EventCountThreshold = 5
	config.RelationChangeThreshold = 2
	config.AnomalyScoreThreshold = 0.7
	pastTime := time.Now().Add(-2 * time.Hour)
	state := ReflectionTriggerState{
		LastReflectionAt:    pastTime,
		EventCountSinceLast: 4,
	}
	result := EvaluateTrigger(state, config, time.Now(), 2, 2, []float64{0.9})
	if !result.Fired {
		t.Error("expected multiple triggers to fire")
	}
	if len(result.Kinds) < 2 {
		t.Errorf("expected at least 2 trigger kinds, got %d", len(result.Kinds))
	}
}

func TestEvaluateTrigger_NoFire(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	config.TimeThreshold = 1 * time.Hour
	config.EventCountThreshold = 100
	config.RelationChangeThreshold = 10
	config.AnomalyScoreThreshold = 0.95
	recentTime := time.Now().Add(-30 * time.Minute)
	state := ReflectionTriggerState{
		LastReflectionAt: recentTime,
	}
	result := EvaluateTrigger(state, config, time.Now(), 5, 1, []float64{0.3})
	if result.Fired {
		t.Error("expected no triggers to fire")
	}
}

func TestResetTriggerState(t *testing.T) {
	state := ResetTriggerState()
	if state.LastReflectionAt.IsZero() {
		t.Error("expected non-zero last reflection time")
	}
	if state.EventCountSinceLast != 0 {
		t.Errorf("expected 0 events, got %d", state.EventCountSinceLast)
	}
	if state.RelationChangeCount != 0 {
		t.Errorf("expected 0 relation changes, got %d", state.RelationChangeCount)
	}
	if state.MaxAnomalyScore != 0 {
		t.Errorf("expected 0 anomaly score, got %f", state.MaxAnomalyScore)
	}
}

func TestDefaultReflectionTriggerConfig(t *testing.T) {
	config := DefaultReflectionTriggerConfig()
	if config.TimeThreshold != 24*time.Hour {
		t.Errorf("expected 24h, got %v", config.TimeThreshold)
	}
	if config.EventCountThreshold != 50 {
		t.Errorf("expected 50, got %d", config.EventCountThreshold)
	}
	if config.RelationChangeThreshold != 3 {
		t.Errorf("expected 3, got %d", config.RelationChangeThreshold)
	}
	if config.AnomalyScoreThreshold != 0.7 {
		t.Errorf("expected 0.7, got %f", config.AnomalyScoreThreshold)
	}
}
