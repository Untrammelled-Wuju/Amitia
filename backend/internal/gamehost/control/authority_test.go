package control

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestCreate_DefaultObserveEpoch1(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-1")
	pluginID := domain.PluginID("plugin-1")

	snap, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if snap.Mode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode %q, got %q", domain.ControlModeObserveOnly, snap.Mode)
	}
	if snap.Epoch != 1 {
		t.Errorf("expected epoch 1, got %d", snap.Epoch)
	}
	if snap.RuntimeID != runtimeID {
		t.Errorf("expected runtime id %q, got %q", runtimeID, snap.RuntimeID)
	}
	if snap.PluginID != pluginID {
		t.Errorf("expected plugin id %q, got %q", pluginID, snap.PluginID)
	}
}

func TestGet_AfterCreate(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-2")
	pluginID := domain.PluginID("plugin-2")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.RuntimeID != runtimeID {
		t.Errorf("expected runtime id %q, got %q", runtimeID, snap.RuntimeID)
	}
	if snap.Mode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode %q, got %q", domain.ControlModeObserveOnly, snap.Mode)
	}
}

func TestCreate_DuplicateAlreadyExists(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-3")
	pluginID := domain.PluginID("plugin-3")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	_, err = m.Create(ctx, runtimeID, pluginID)
	if err == nil {
		t.Fatal("expected error for duplicate Create, got nil")
	}
}

func TestGet_NotFound(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()

	_, err := m.Get(ctx, domain.RuntimeInstanceID("nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent runtime, got nil")
	}
}

func TestTransition_ObserveToPlugin(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-4")
	pluginID := domain.PluginID("plugin-4")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	snap, err := m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}
	if snap.Mode != domain.ControlModePluginControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModePluginControl, snap.Mode)
	}
	if snap.Epoch != 2 {
		t.Errorf("expected epoch 2, got %d", snap.Epoch)
	}
	if snap.LastTransitionActor != ActorPlugin {
		t.Errorf("expected actor %q, got %q", ActorPlugin, snap.LastTransitionActor)
	}
	if snap.LastTransitionReason != ReasonPluginRequest {
		t.Errorf("expected reason %q, got %q", ReasonPluginRequest, snap.LastTransitionReason)
	}
}

func TestTransition_SameStateNoop(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-5")
	pluginID := domain.PluginID("plugin-5")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	snap, err := m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeObserveOnly,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}
	if snap.Mode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode %q, got %q", domain.ControlModeObserveOnly, snap.Mode)
	}
	if snap.Epoch != 1 {
		t.Errorf("expected epoch 1 (unchanged), got %d", snap.Epoch)
	}
}

func TestTransition_EpochIncrement(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-6")
	pluginID := domain.PluginID("plugin-6")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	modes := []domain.ControlMode{
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
		domain.ControlModeSharedControl,
		domain.ControlModeAssist,
		domain.ControlModeObserveOnly,
	}

	for i, mode := range modes {
		expectedEpoch := uint64(i + 2)
		snap, err := m.Transition(ctx, runtimeID, TransitionRequest{
			Target: mode,
			Actor:  ActorHost,
			Reason: ReasonHostPolicy,
		})
		if err != nil {
			t.Fatalf("Transition %d -> %q failed: %v", i, mode, err)
		}
		if snap.Epoch != expectedEpoch {
			t.Errorf("iteration %d: expected epoch %d, got %d", i, expectedEpoch, snap.Epoch)
		}
	}
}

func TestTransition_ExpectedEpochSuccess(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-7")
	pluginID := domain.PluginID("plugin-7")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	snap, err := m.Transition(ctx, runtimeID, TransitionRequest{
		Target:        domain.ControlModePluginControl,
		Actor:         ActorPlugin,
		Reason:        ReasonPluginRequest,
		ExpectedEpoch: 1,
		UseExpected:   true,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}
	if snap.Epoch != 2 {
		t.Errorf("expected epoch 2, got %d", snap.Epoch)
	}
}

func TestTransition_ExpectedEpochFailure(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-8")
	pluginID := domain.PluginID("plugin-8")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target:        domain.ControlModePluginControl,
		Actor:         ActorPlugin,
		Reason:        ReasonPluginRequest,
		ExpectedEpoch: 999,
		UseExpected:   true,
	})
	if err == nil {
		t.Fatal("expected stale epoch error, got nil")
	}

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Epoch != 1 {
		t.Errorf("expected epoch unchanged at 1, got %d", snap.Epoch)
	}
	if snap.Mode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode unchanged at %q, got %q", domain.ControlModeObserveOnly, snap.Mode)
	}
}

func TestTransition_InvalidMode(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-invalid")
	pluginID := domain.PluginID("plugin-invalid")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlMode("nonexistent_mode"),
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})
	if err == nil {
		t.Fatal("expected invalid mode error, got nil")
	}
}

func TestTransition_RuntimeNotFound(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()

	_, err := m.Transition(ctx, domain.RuntimeInstanceID("nonexistent"), TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})
	if err == nil {
		t.Fatal("expected runtime not found error, got nil")
	}
}

func TestRemove_Cleanup(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-remove")
	pluginID := domain.PluginID("plugin-remove")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = m.Remove(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err = m.Get(ctx, runtimeID)
	if err == nil {
		t.Fatal("expected error after Remove, got nil")
	}

	m.CleanupRuntimeLock(runtimeID)
}

func TestList_SortedByRuntimeID(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()

	ids := []domain.RuntimeInstanceID{
		"rt-z",
		"rt-a",
		"rt-m",
	}
	for _, id := range ids {
		_, err := m.Create(ctx, id, domain.PluginID("plugin-"+string(id)))
		if err != nil {
			t.Fatalf("Create failed for %q: %v", id, err)
		}
	}

	list, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list))
	}

	expectedOrder := []domain.RuntimeInstanceID{"rt-a", "rt-m", "rt-z"}
	for i, expected := range expectedOrder {
		if list[i].RuntimeID != expected {
			t.Errorf("position %d: expected %q, got %q", i, expected, list[i].RuntimeID)
		}
	}
}

func TestClock_Custom(t *testing.T) {
	fixedTime := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{
		Clock: func() time.Time { return fixedTime },
	})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-clock")
	pluginID := domain.PluginID("plugin-clock")

	snap, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !snap.UpdatedAt.Equal(fixedTime) {
		t.Errorf("expected updated at %v, got %v", fixedTime, snap.UpdatedAt)
	}
}

func TestTransition_EpochNeverDecrements(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-epoch")
	pluginID := domain.PluginID("plugin-epoch")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition 1 failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})
	if err != nil {
		t.Fatalf("Transition 2 failed: %v", err)
	}

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Epoch != 3 {
		t.Errorf("expected epoch 3, got %d", snap.Epoch)
	}
}

func TestAllControlModes_Supported(t *testing.T) {
	modes := []domain.ControlMode{
		domain.ControlModeObserveOnly,
		domain.ControlModeAssist,
		domain.ControlModeSharedControl,
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended,
	}

	for _, mode := range modes {
		if !IsValidControlMode(mode) {
			t.Errorf("mode %q should be valid", mode)
		}
	}
}

func TestCanTransition_AllAllowedTransitions(t *testing.T) {
	allModes := []domain.ControlMode{
		domain.ControlModeObserveOnly,
		domain.ControlModeAssist,
		domain.ControlModeSharedControl,
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended,
	}

	for _, from := range allModes {
		for _, to := range allModes {
			if from == to {
				continue
			}
			if !CanTransition(from, to) {
				t.Errorf("expected transition %q -> %q to be allowed", from, to)
			}
		}
	}
}

func TestCanTransition_SameModeAlwaysFalse(t *testing.T) {
	modes := []domain.ControlMode{
		domain.ControlModeObserveOnly,
		domain.ControlModeAssist,
		domain.ControlModeSharedControl,
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended,
	}

	for _, mode := range modes {
		if CanTransition(mode, mode) {
			t.Errorf("expected same-mode transition %q -> %q to be false", mode, mode)
		}
	}
}

func TestTransition_FullCycle(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-cycle")
	pluginID := domain.PluginID("plugin-cycle")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	chain := []domain.ControlMode{
		domain.ControlModePluginControl,
		domain.ControlModeSharedControl,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended,
		domain.ControlModeObserveOnly,
	}

	for i, target := range chain {
		expectedEpoch := uint64(i + 2)
		snap, err := m.Transition(ctx, runtimeID, TransitionRequest{
			Target: target,
			Actor:  ActorHost,
			Reason: ReasonRuntimeLifecycle,
		})
		if err != nil {
			t.Fatalf("Transition %d to %q failed: %v", i, target, err)
		}
		if snap.Epoch != expectedEpoch {
			t.Errorf("iteration %d: expected epoch %d, got %d", i, expectedEpoch, snap.Epoch)
		}
	}
}
