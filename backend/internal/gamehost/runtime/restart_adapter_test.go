package runtime

import (
	"context"
	"testing"
	"time"
)

func TestRestartAdapter_HandleRestartEvent_Scheduled(t *testing.T) {
	adapter := NewRestartAdapter()

	ctx := context.Background()
	err := adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Generation: 1,
		Event:      RestartScheduled,
		Scheduled:  time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, ok := adapter.GetRestartSnapshot("rt-1", "bridge")
	if !ok {
		t.Fatal("expected restart snapshot")
	}
	if !snap.Restarting {
		t.Error("expected restarting state for scheduled event")
	}
	if snap.Exhausted {
		t.Error("scheduled event should not mark as exhausted")
	}
}

func TestRestartAdapter_HandleRestartEvent_Started(t *testing.T) {
	adapter := NewRestartAdapter()

	ctx := context.Background()
	now := time.Now()

	err := adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Generation: 1,
		Event:      RestartStarted,
		Scheduled:  now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, ok := adapter.GetRestartSnapshot("rt-1", "bridge")
	if !ok {
		t.Fatal("expected restart snapshot")
	}
	if snap.RestartCount != 1 {
		t.Errorf("expected restart count 1, got %d", snap.RestartCount)
	}
	if snap.LastRestartAt == nil {
		t.Error("expected LastRestartAt to be set")
	}
}

func TestRestartAdapter_HandleRestartEvent_Succeeded(t *testing.T) {
	adapter := NewRestartAdapter()

	ctx := context.Background()
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Event: RestartStarted,
	})

	err := adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Event: RestartSucceeded,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, ok := adapter.GetRestartSnapshot("rt-1", "bridge")
	if !ok {
		t.Fatal("expected restart snapshot")
	}
	if snap.Restarting {
		t.Error("restart should not be in-progress after success")
	}
	if snap.Exhausted {
		t.Error("should not be exhausted after success")
	}
}

func TestRestartAdapter_HandleRestartEvent_Exhausted(t *testing.T) {
	adapter := NewRestartAdapter()

	ctx := context.Background()
	err := adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Event: RestartExhausted, Reason: "max_restarts_reached",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, ok := adapter.GetRestartSnapshot("rt-1", "bridge")
	if !ok {
		t.Fatal("expected restart snapshot")
	}
	if !snap.Exhausted {
		t.Error("expected exhausted state")
	}
	if snap.Restarting {
		t.Error("should not be restarting after exhausted")
	}
	if snap.Reason != "max_restarts_reached" {
		t.Errorf("expected reason preserved, got %s", snap.Reason)
	}
}

func TestRestartAdapter_ListRestartSnapshots_Sorted(t *testing.T) {
	adapter := NewRestartAdapter()

	ctx := context.Background()
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "z-service", Event: RestartScheduled,
	})
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "a-service", Event: RestartScheduled,
	})

	list := adapter.ListRestartSnapshots("rt-1")
	if len(list) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(list))
	}
	if list[0].ServiceID != "a-service" || list[1].ServiceID != "z-service" {
		t.Errorf("expected sorted order, got [%s, %s]", list[0].ServiceID, list[1].ServiceID)
	}
}

func TestRestartAdapter_RemoveService(t *testing.T) {
	adapter := NewRestartAdapter()
	r := adapter.(*restartAdapter)

	ctx := context.Background()
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Event: RestartScheduled,
	})

	r.RemoveService("rt-1", "bridge")

	if _, ok := adapter.GetRestartSnapshot("rt-1", "bridge"); ok {
		t.Error("expected service to be removed")
	}
}

func TestRestartAdapter_RemoveRuntime(t *testing.T) {
	adapter := NewRestartAdapter()
	r := adapter.(*restartAdapter)

	ctx := context.Background()
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Event: RestartScheduled,
	})

	r.RemoveRuntime("rt-1")

	if list := adapter.ListRestartSnapshots("rt-1"); len(list) != 0 {
		t.Error("expected runtime to be removed")
	}
}

func TestRestartAdapter_MultipleGenerations(t *testing.T) {
	adapter := NewRestartAdapter()

	ctx := context.Background()

	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Generation: 1, Event: RestartStarted,
	})
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Event: RestartFailed,
	})
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Generation: 2, Event: RestartStarted,
	})
	_ = adapter.HandleRestartEvent(ctx, SupervisorRestartEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Generation: 2, Event: RestartSucceeded,
	})

	snap, ok := adapter.GetRestartSnapshot("rt-1", "bridge")
	if !ok {
		t.Fatal("expected restart snapshot")
	}
	if snap.Generation != 2 {
		t.Errorf("expected generation 2, got %d", snap.Generation)
	}
	if snap.RestartCount != 2 {
		t.Errorf("expected restart count 2, got %d", snap.RestartCount)
	}
}

func TestRestartSnapshot_Clone(t *testing.T) {
	now := time.Now()
	original := RestartSnapshot{
		RuntimeID:     "rt-1",
		ServiceID:     "bridge",
		Generation:    1,
		RestartCount:  3,
		LastRestartAt: &now,
		Restarting:    true,
		Reason:        "testing",
	}

	clone := original.Clone()
	clone.Generation = 99
	clone.Reason = "changed"

	if original.Generation != 1 {
		t.Error("clone mutation should not affect original")
	}
	if original.Reason != "testing" {
		t.Error("clone Reason mutation should not affect original")
	}
}
