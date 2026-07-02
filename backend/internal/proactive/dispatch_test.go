package proactive

import (
	"context"
	"testing"
)

func TestDispatchAllWithContextStopsWhenCancelled(t *testing.T) {
	var scheduleCalls int
	var burstCalls int
	RegisterCompanionDispatchContext(
		func(ctx context.Context, date string, characterID string) interface{} {
			scheduleCalls++
			return nil
		},
		nil,
		func(ctx context.Context, characterID string) interface{} {
			burstCalls++
			return nil
		},
	)
	t.Cleanup(func() {
		RegisterCompanionDispatchContext(nil, nil, nil)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	DispatchAllWithContext(ctx, "2026-07-02", []string{"char-1", "char-2"})

	if scheduleCalls != 0 {
		t.Fatalf("expected no schedule calls after cancellation, got %d", scheduleCalls)
	}
	if burstCalls != 0 {
		t.Fatalf("expected no burst calls after cancellation, got %d", burstCalls)
	}
}

func TestDispatchDueTasksWithContextReturnsCancelledStatus(t *testing.T) {
	var calls int
	RegisterCompanionDispatchContext(nil, func(ctx context.Context, characterID string) interface{} {
		calls++
		return map[string]interface{}{"status": "sent"}
	}, nil)
	t.Cleanup(func() {
		RegisterCompanionDispatchContext(nil, nil, nil)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := DispatchDueTasksWithContext(ctx, "char-1")
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %#v", result)
	}
	if m["status"] != "cancelled" {
		t.Fatalf("expected cancelled status, got %#v", result)
	}
	if calls != 0 {
		t.Fatalf("expected no due processor calls after cancellation, got %d", calls)
	}
}
