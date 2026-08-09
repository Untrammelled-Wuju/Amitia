package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestQuarantineAdapter_HandleQuarantineEvent(t *testing.T) {
	adapter := NewQuarantineAdapter()

	ctx := context.Background()
	err := adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID:    "rt-1",
		ServiceID:    "bridge",
		Generation:   1,
		Quarantined:  true,
		Reason:       "frequent_crash",
		Occurred:     time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !adapter.IsQuarantined("rt-1", "bridge") {
		t.Error("expected service to be quarantined")
	}

	snap, ok := adapter.GetQuarantineSnapshot("rt-1", "bridge")
	if !ok {
		t.Fatal("expected quarantine snapshot")
	}
	if !snap.Quarantined {
		t.Error("expected Quarantined=true")
	}
	if snap.Since == nil {
		t.Error("expected Since to be set")
	}
	if snap.Reason != "frequent_crash" {
		t.Errorf("expected reason preserved, got %s", snap.Reason)
	}
}

func TestQuarantineAdapter_Release(t *testing.T) {
	adapter := NewQuarantineAdapter()
	r := adapter.(*quarantineAdapter)

	ctx := context.Background()
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Quarantined: true,
	})

	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Quarantined: false,
	})

	if adapter.IsQuarantined("rt-1", "bridge") {
		t.Error("expected service to be released")
	}

	if _, ok := adapter.GetQuarantineSnapshot("rt-1", "bridge"); ok {
		t.Error("expected snapshot to be removed after release")
	}

	_ = r
}

func TestQuarantineAdapter_ListQuarantineSnapshots(t *testing.T) {
	adapter := NewQuarantineAdapter()

	ctx := context.Background()
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Quarantined: true, Reason: "crash_loop",
	})
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "agent", Quarantined: true, Reason: "quarantine_fail",
	})

	list := adapter.ListQuarantineSnapshots("rt-1")
	if len(list) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(list))
	}

	if list[0].ServiceID != "agent" || list[1].ServiceID != "bridge" {
		t.Errorf("expected sorted order, got [%s, %s]", list[0].ServiceID, list[1].ServiceID)
	}
}

func TestQuarantineAdapter_RemoveService(t *testing.T) {
	adapter := NewQuarantineAdapter()
	r := adapter.(*quarantineAdapter)

	ctx := context.Background()
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Quarantined: true,
	})

	r.RemoveService("rt-1", "bridge")

	if adapter.IsQuarantined("rt-1", "bridge") {
		t.Error("expected service to be removed")
	}
}

func TestQuarantineSnapshot_Clone(t *testing.T) {
	now := time.Now()
	original := QuarantineSnapshot{
		RuntimeID:   "rt-1",
		ServiceID:   "bridge",
		Quarantined: true,
		Since:       &now,
		Reason:      "crash_loop",
	}

	clone := original.Clone()
	clone.Quarantined = false
	clone.Reason = "changed"
	newTime := now.Add(time.Hour)
	clone.Since = &newTime

	if !original.Quarantined {
		t.Error("clone mutation should not affect original")
	}
	if original.Reason != "crash_loop" {
		t.Error("clone Reason mutation should not affect original")
	}
}

func TestQuarantineAdapter_RuntimeIsolation(t *testing.T) {
	adapter := NewQuarantineAdapter()

	ctx := context.Background()
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Quarantined: true,
	})

	if adapter.IsQuarantined("rt-2", "bridge") {
		t.Error("quarantine should be per-runtime")
	}
}

func TestQuarantineAdapter_MultiRuntimeSameDefinition(t *testing.T) {
	adapter := NewQuarantineAdapter()

	ctx := context.Background()
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Quarantined: true,
	})
	_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
		RuntimeID: "rt-2", ServiceID: "bridge", Quarantined: false,
	})

	if !adapter.IsQuarantined("rt-1", "bridge") {
		t.Error("rt-1 should be quarantined")
	}
	if adapter.IsQuarantined("rt-2", "bridge") {
		t.Error("rt-2 should not be quarantined")
	}
}

func TestQuarantineReasonFromTrustedService(t *testing.T) {
	tests := map[string]string{
		"frequent_crash":           "frequent_crash",
		"signature_failure":        "signature_failure",
		"process_tree_unkillable":  "process_tree_unkillable",
	}

	for input, expected := range tests {
		_ = domain.RuntimeInstanceID(input)
		adapter := NewQuarantineAdapter()
		ctx := context.Background()
		_ = adapter.HandleQuarantineEvent(ctx, SupervisorQuarantineEvent{
			RuntimeID: "rt-1", ServiceID: "svc", Quarantined: true, Reason: input,
		})
		snap, _ := adapter.GetQuarantineSnapshot("rt-1", "svc")
		if snap.Reason != expected {
			t.Errorf("expected reason %s, got %s", expected, snap.Reason)
		}
	}
}
