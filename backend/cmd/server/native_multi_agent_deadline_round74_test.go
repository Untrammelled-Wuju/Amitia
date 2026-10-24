package main

import (
	"testing"
	"time"
)

func TestRound74DeadlineMinuteBounds(t *testing.T) {
	if err := validateMultiAgentDeadlineMinutes(nil); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		minutes int
		allowed bool
	}{
		{-1, false}, {0, false}, {1, true}, {15, true}, {240, true}, {241, false},
	} {
		got := validateMultiAgentDeadlineMinutes(&item.minutes)
		if (got == nil) != item.allowed {
			t.Fatalf("deadline %d allowed=%t: %v", item.minutes, item.allowed, got)
		}
	}
}

func TestRound74DeadlineUsesAbsoluteUTCInstant(t *testing.T) {
	now := time.Date(2026, 10, 23, 3, 59, 30, 0, time.FixedZone("test", 8*3600))
	deadline := multiAgentDeadlineFromMinutes(now, 15)
	if !deadline.Equal(now.Add(15*time.Minute)) || deadline.Location() != time.UTC {
		t.Fatalf("incorrect UTC absolute deadline: %v", deadline)
	}
}
