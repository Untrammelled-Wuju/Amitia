package mindruntime

import (
	"math"
	"testing"
)

func TestNewGrowthTracker_Defaults(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	tracker := NewGrowthTracker("char-1", config)
	if tracker.CharacterID != "char-1" {
		t.Errorf("expected char-1, got %s", tracker.CharacterID)
	}
	if tracker.MessageCount != 0 {
		t.Errorf("expected 0 messages, got %d", tracker.MessageCount)
	}
	if !tracker.Enabled {
		t.Error("expected Enabled true by default")
	}
	if len(tracker.Parameters) != 5 {
		t.Errorf("expected 5 parameters, got %d", len(tracker.Parameters))
	}
}

func TestNewGrowthTracker_Disabled(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.Enabled = false
	tracker := NewGrowthTracker("char-1", config)
	if tracker.Enabled {
		t.Error("expected Enabled false")
	}
}

func TestRecordMessages_InsufficientCount(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.MessageInterval = 500
	tracker := NewGrowthTracker("char-1", config)
	deltas, grew := tracker.RecordMessages(100, config)
	if grew {
		t.Error("expected no growth with insufficient messages")
	}
	if len(deltas) != 0 {
		t.Errorf("expected 0 deltas, got %d", len(deltas))
	}
}

func TestRecordMessages_WhenDisabled(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.Enabled = false
	tracker := NewGrowthTracker("char-1", config)
	tracker.Enabled = false
	_, grew := tracker.RecordMessages(500, config)
	if grew {
		t.Error("expected no growth when disabled")
	}
}

func TestRecordMessages_TriggersGrowth(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.MessageInterval = 200
	config.GrowthRate = 0.1
	config.DecayFactor = 1.0
	config.MaxTotalChange = 1.0
	tracker := NewGrowthTracker("char-1", config)
	_, grew := tracker.RecordMessages(200, config)
	if !grew {
		t.Error("expected growth after 200 messages")
	}
	if len(tracker.GrowthHistory) != 1 {
		t.Errorf("expected 1 growth record, got %d", len(tracker.GrowthHistory))
	}
}

func TestRecordMessages_GrowthIsSlow(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.MessageInterval = 500
	config.GrowthRate = 0.05
	config.DecayFactor = 1.0
	config.MaxTotalChange = 1.0
	tracker := NewGrowthTracker("char-1", config)
	_, grew := tracker.RecordMessages(500, config)
	if !grew {
		t.Fatal("expected growth")
	}
	for _, p := range tracker.Parameters {
		if math.Abs(p.Current-0.5) > 0.01 {
			t.Errorf("expected very small change for %s, current=%f", p.Name, p.Current)
		}
	}
}

func TestRecordMessages_HundredsOfMessages(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.MessageInterval = 200
	config.GrowthRate = 0.01
	config.DecayFactor = 1.0
	config.MaxTotalChange = 1.0
	tracker := NewGrowthTracker("char-1", config)
	_, grew := tracker.RecordMessages(600, config)
	if !grew {
		t.Fatal("expected growth after 600 messages")
	}
	if tracker.MessageCount != 0 {
		t.Errorf("expected residual count 0 after cleanup, got %d", tracker.MessageCount)
	}
}

func TestGetParameter_Found(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	tracker := NewGrowthTracker("char-1", config)
	p, ok := tracker.GetParameter("expressiveness")
	if !ok {
		t.Error("expected to find expressiveness parameter")
	}
	if p.Current != 0.5 {
		t.Errorf("expected 0.5, got %f", p.Current)
	}
}

func TestGetParameter_NotFound(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	tracker := NewGrowthTracker("char-1", config)
	_, ok := tracker.GetParameter("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent parameter")
	}
}

func TestGetAllParameters(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	tracker := NewGrowthTracker("char-1", config)
	params := tracker.GetAllParameters()
	if len(params) != 5 {
		t.Errorf("expected 5 params, got %d", len(params))
	}
}

func TestGetTotalChange(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	tracker := NewGrowthTracker("char-1", config)
	if tracker.GetTotalChange() != 0 {
		t.Errorf("expected 0 total change initially, got %f", tracker.GetTotalChange())
	}
}

func TestIsAtTarget(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	config.GrowthRate = 1.0
	config.DecayFactor = 1.0
	config.MaxTotalChange = 5.0
	config.MessageInterval = 10
	tracker := NewGrowthTracker("char-1", config)
	if tracker.IsAtTarget(config) {
		t.Error("expected not at target initially")
	}
	tracker.RecordMessages(10000, config)
	if !tracker.IsAtTarget(config) {
		t.Error("expected at target after massive growth")
	}
}

func TestDisableAndEnable(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	tracker := NewGrowthTracker("char-1", config)
	tracker.Disable()
	if tracker.Enabled {
		t.Error("expected disabled after Disable()")
	}
	tracker.Enable()
	if !tracker.Enabled {
		t.Error("expected enabled after Enable()")
	}
}

func TestDefaultPersonalityGrowthConfig(t *testing.T) {
	config := DefaultPersonalityGrowthConfig()
	if !config.Enabled {
		t.Error("expected Enabled true")
	}
	if config.GrowthRate != 0.001 {
		t.Errorf("expected 0.001, got %f", config.GrowthRate)
	}
	if config.MessageInterval != 200 {
		t.Errorf("expected 200, got %d", config.MessageInterval)
	}
	if config.DecayFactor != 0.95 {
		t.Errorf("expected 0.95, got %f", config.DecayFactor)
	}
	if config.MaxTotalChange != 0.3 {
		t.Errorf("expected 0.3, got %f", config.MaxTotalChange)
	}
}
