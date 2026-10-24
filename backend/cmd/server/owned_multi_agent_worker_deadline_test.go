package main

import (
	"strings"
	"testing"
	"time"
)

func TestRound75OwnedWorkerEffectiveDeadline(t *testing.T) {
	now := time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		parent *time.Time
		want   time.Time
		reject bool
	}{
		{name: "absent durable budget uses bounded worker attempt", want: now.Add(30 * time.Minute)},
		{name: "earlier durable deadline takes precedence", parent: timePointer75(now.Add(90 * time.Second)), want: now.Add(90 * time.Second)},
		{name: "later durable deadline does not extend attempt", parent: timePointer75(now.Add(4 * time.Hour)), want: now.Add(30 * time.Minute)},
		{name: "expired parent is rejected", parent: timePointer75(now.Add(-time.Second)), reject: true},
		{name: "exactly due parent is rejected", parent: timePointer75(now), reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ownedWorkerEffectiveDeadline(now, tc.parent)
			if tc.reject {
				if err == nil || !strings.Contains(err.Error(), "deadline") {
					t.Fatalf("expected expiry error: %v", err)
				}
				return
			}
			if err != nil || !got.Equal(tc.want) {
				t.Fatalf("deadline = %v, error = %v; want %v", got, err, tc.want)
			}
		})
	}
}

func timePointer75(value time.Time) *time.Time { return &value }
