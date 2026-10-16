package main

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/reminder"
)

func TestProactiveLevelChangesActualCooldownAndPreservesDefault(t *testing.T) {
	for _, test := range []struct {
		level    int
		expected time.Duration
	}{{40, time.Hour}, {80, 30 * time.Minute}, {20, 2 * time.Hour}} {
		policy := &reminder.ProactivePolicy{ActiveLevel: test.level, MinInterval: 60}
		if actual := proactiveMinimumInterval(policy); actual != test.expected {
			t.Fatalf("level %d: cooldown=%s want=%s", test.level, actual, test.expected)
		}
	}
}
