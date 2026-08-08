package domain

import (
	"testing"
	"time"
)

func TestCanTransitionRuntimeState_ValidTransitions(t *testing.T) {
	validCases := []struct {
		from RuntimeState
		to   RuntimeState
	}{
		{RuntimeStateCreated, RuntimeStateStarting},
		{RuntimeStateStarting, RuntimeStateRunning},
		{RuntimeStateStarting, RuntimeStateDegraded},
		{RuntimeStateStarting, RuntimeStateStopping},
		{RuntimeStateStarting, RuntimeStateFailed},
		{RuntimeStateRunning, RuntimeStateDegraded},
		{RuntimeStateRunning, RuntimeStateSuspended},
		{RuntimeStateRunning, RuntimeStateStopping},
		{RuntimeStateRunning, RuntimeStateFailed},
		{RuntimeStateDegraded, RuntimeStateRunning},
		{RuntimeStateDegraded, RuntimeStateSuspended},
		{RuntimeStateDegraded, RuntimeStateStopping},
		{RuntimeStateDegraded, RuntimeStateFailed},
		{RuntimeStateSuspended, RuntimeStateRunning},
		{RuntimeStateSuspended, RuntimeStateDegraded},
		{RuntimeStateSuspended, RuntimeStateStopping},
		{RuntimeStateSuspended, RuntimeStateFailed},
		{RuntimeStateStopping, RuntimeStateStopped},
		{RuntimeStateStopping, RuntimeStateFailed},
	}

	for _, tc := range validCases {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if !CanTransitionRuntimeState(tc.from, tc.to) {
				t.Errorf("expected %s -> %s to be valid", tc.from, tc.to)
			}
		})
	}
}

func TestCanTransitionRuntimeState_InvalidTransitions(t *testing.T) {
	invalidCases := []struct {
		from RuntimeState
		to   RuntimeState
	}{
		{RuntimeStateCreated, RuntimeStateRunning},
		{RuntimeStateCreated, RuntimeStateStopped},
		{RuntimeStateCreated, RuntimeStateFailed},
		{RuntimeStateStopped, RuntimeStateRunning},
		{RuntimeStateStopped, RuntimeStateStarting},
		{RuntimeStateStopped, RuntimeStateCreated},
		{RuntimeStateFailed, RuntimeStateRunning},
		{RuntimeStateFailed, RuntimeStateStarting},
		{RuntimeStateFailed, RuntimeStateCreated},
		{RuntimeStateRunning, RuntimeStateCreated},
		{RuntimeStateRunning, RuntimeStateStarting},
		{RuntimeStateSuspended, RuntimeStateCreated},
		{RuntimeStateRunning, RuntimeStateRunning},
		{RuntimeStateStopped, RuntimeStateStopped},
	}

	for _, tc := range invalidCases {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if CanTransitionRuntimeState(tc.from, tc.to) {
				t.Errorf("expected %s -> %s to be invalid", tc.from, tc.to)
			}
		})
	}
}

func TestIsTerminalRuntimeState(t *testing.T) {
	if !IsTerminalRuntimeState(RuntimeStateStopped) {
		t.Error("expected stopped to be terminal")
	}
	if !IsTerminalRuntimeState(RuntimeStateFailed) {
		t.Error("expected failed to be terminal")
	}
	if IsTerminalRuntimeState(RuntimeStateRunning) {
		t.Error("expected running to not be terminal")
	}
	if IsTerminalRuntimeState(RuntimeStateSuspended) {
		t.Error("expected suspended to not be terminal")
	}
	if IsTerminalRuntimeState(RuntimeStateCreated) {
		t.Error("expected created to not be terminal")
	}
}

func TestIsActiveRuntimeState(t *testing.T) {
	activeStates := []RuntimeState{
		RuntimeStateStarting,
		RuntimeStateRunning,
		RuntimeStateDegraded,
		RuntimeStateSuspended,
		RuntimeStateStopping,
	}
	inactiveStates := []RuntimeState{
		RuntimeStateCreated,
		RuntimeStateStopped,
		RuntimeStateFailed,
	}

	for _, s := range activeStates {
		if !IsActiveRuntimeState(s) {
			t.Errorf("expected %s to be active", s)
		}
	}
	for _, s := range inactiveStates {
		if IsActiveRuntimeState(s) {
			t.Errorf("expected %s to not be active", s)
		}
	}
}

func TestIsValidRuntimeState(t *testing.T) {
	for _, state := range AllRuntimeStates() {
		if !IsValidRuntimeState(state) {
			t.Errorf("expected %s to be a valid state", state)
		}
	}
	if IsValidRuntimeState(RuntimeState("banana")) {
		t.Error("expected banana to be invalid")
	}
	if IsValidRuntimeState(RuntimeState("")) {
		t.Error("expected empty string to be invalid")
	}
}

func TestNewRuntimeInstance(t *testing.T) {
	now := time.Now()
	inst, err := NewRuntimeInstance("rt-001", "plugin-a", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.ID != "rt-001" {
		t.Errorf("unexpected ID: %s", inst.ID)
	}
	if inst.PluginID != "plugin-a" {
		t.Errorf("unexpected PluginID: %s", inst.PluginID)
	}
	if inst.State != RuntimeStateCreated {
		t.Errorf("expected state created, got %s", inst.State)
	}
	if inst.Health.Status != HealthUnknown {
		t.Errorf("expected health unknown, got %s", inst.Health.Status)
	}
	if !inst.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, inst.CreatedAt)
	}
	if !inst.UpdatedAt.Equal(now) {
		t.Errorf("expected UpdatedAt %v, got %v", now, inst.UpdatedAt)
	}
	if inst.StartedAt != nil {
		t.Error("expected StartedAt to be nil")
	}
	if inst.StoppedAt != nil {
		t.Error("expected StoppedAt to be nil")
	}
	if inst.SuspendedAt != nil {
		t.Error("expected SuspendedAt to be nil")
	}
	if inst.FailedAt != nil {
		t.Error("expected FailedAt to be nil")
	}
	if inst.StateReason != "" {
		t.Errorf("expected empty StateReason, got %s", inst.StateReason)
	}
}

func TestNewRuntimeInstanceRejectsEmptyID(t *testing.T) {
	now := time.Now()
	_, err := NewRuntimeInstance("", "plugin-a", now)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument error, got %v", err)
	}
}

func TestNewRuntimeInstanceRejectsEmptyPluginID(t *testing.T) {
	now := time.Now()
	_, err := NewRuntimeInstance("rt-001", "", now)
	if err == nil {
		t.Fatal("expected error for empty PluginID")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument error, got %v", err)
	}
}

func TestTransition_Success(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	later := now.Add(time.Second)
	err := inst.Transition(RuntimeStateStarting, "begin_startup", later)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.State != RuntimeStateStarting {
		t.Errorf("expected state starting, got %s", inst.State)
	}
	if inst.StateReason != "begin_startup" {
		t.Errorf("expected StateReason 'begin_startup', got %s", inst.StateReason)
	}
	if !inst.UpdatedAt.Equal(later) {
		t.Errorf("expected UpdatedAt to be updated")
	}
}

func TestValidRuntimeTransitions(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	path := []struct {
		state  RuntimeState
		reason string
	}{
		{RuntimeStateStarting, "begin_startup"},
		{RuntimeStateRunning, "startup_complete"},
		{RuntimeStateDegraded, "service_unhealthy"},
		{RuntimeStateRunning, "service_recovered"},
		{RuntimeStateSuspended, "user_takeover"},
		{RuntimeStateRunning, "user_release"},
		{RuntimeStateStopping, "runtime_shutdown"},
		{RuntimeStateStopped, "shutdown_complete"},
	}

	for i, step := range path {
		later := now.Add(time.Duration(i+1) * time.Second)
		err := inst.Transition(step.state, step.reason, later)
		if err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
		if inst.State != step.state {
			t.Errorf("step %d: expected state %s, got %s", i, step.state, inst.State)
		}
	}
}

func TestInvalidRuntimeTransitions(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	invalidTargets := []RuntimeState{
		RuntimeStateRunning,
		RuntimeStateStopped,
		RuntimeStateFailed,
	}

	for _, target := range invalidTargets {
		err := inst.Transition(target, "test", now.Add(time.Second))
		if err == nil {
			t.Errorf("expected error when transitioning from created to %s", target)
		}
	}
}

func TestTransitionFailureDoesNotMutate(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	originalState := inst.State
	originalUpdatedAt := inst.UpdatedAt
	originalStateReason := inst.StateReason
	originalStartedAt := inst.StartedAt

	err := inst.Transition(RuntimeStateRunning, "should_fail", now.Add(time.Second))
	if err == nil {
		t.Fatal("expected transition to fail")
	}

	if inst.State != originalState {
		t.Errorf("State should not change: got %s, want %s", inst.State, originalState)
	}
	if !inst.UpdatedAt.Equal(originalUpdatedAt) {
		t.Errorf("UpdatedAt should not change")
	}
	if inst.StateReason != originalStateReason {
		t.Errorf("StateReason should not change")
	}
	if inst.StartedAt != originalStartedAt {
		t.Errorf("StartedAt should not change")
	}
}

func TestStartedAtSetOnce(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	t1 := now.Add(time.Second)
	inst.Transition(RuntimeStateStarting, "begin", t1)
	inst.Transition(RuntimeStateRunning, "up", t1)

	if inst.StartedAt == nil {
		t.Fatal("StartedAt should be set after first running")
	}
	firstStartedAt := *inst.StartedAt

	t2 := now.Add(2 * time.Second)
	inst.Transition(RuntimeStateDegraded, "issue", t2)
	inst.Transition(RuntimeStateRunning, "recovered", t2)

	if inst.StartedAt == nil {
		t.Fatal("StartedAt should still be set")
	}
	if *inst.StartedAt != firstStartedAt {
		t.Errorf("StartedAt should not be overwritten: got %v, want %v", *inst.StartedAt, firstStartedAt)
	}
}

func TestStoppedAtOnlyOnStopped(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	t1 := now.Add(time.Second)
	inst.Transition(RuntimeStateStarting, "begin", t1)
	inst.Transition(RuntimeStateStopping, "shutdown", t1)

	t2 := now.Add(2 * time.Second)
	inst.Transition(RuntimeStateFailed, "crash", t2)

	if inst.StoppedAt != nil {
		t.Error("StoppedAt should not be set when transitioning to failed")
	}
	if inst.FailedAt == nil {
		t.Error("FailedAt should be set when transitioning to failed")
	}
}

func TestSuspendedAt(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	t1 := now.Add(time.Second)
	inst.Transition(RuntimeStateStarting, "begin", t1)
	inst.Transition(RuntimeStateRunning, "up", t1)

	t2 := now.Add(2 * time.Second)
	inst.Transition(RuntimeStateSuspended, "user_takeover", t2)

	if inst.SuspendedAt == nil {
		t.Fatal("SuspendedAt should be set when entering suspended")
	}
	if !inst.SuspendedAt.Equal(t2) {
		t.Errorf("expected SuspendedAt %v, got %v", t2, *inst.SuspendedAt)
	}
}

func TestHealthUpdateDoesNotChangeRuntimeState(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	t1 := now.Add(time.Second)
	inst.Transition(RuntimeStateStarting, "begin", t1)
	inst.Transition(RuntimeStateRunning, "up", t1)

	stateBefore := inst.State

	t2 := now.Add(2 * time.Second)
	health := HealthState{
		Status:  HealthUnhealthy,
		Message: "service X down",
	}
	err := inst.UpdateHealth(health, t2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.State != stateBefore {
		t.Errorf("state should not change after health update: got %s, want %s", inst.State, stateBefore)
	}
	if inst.Health.Status != HealthUnhealthy {
		t.Errorf("expected health unhealthy, got %s", inst.Health.Status)
	}
	if !inst.UpdatedAt.Equal(t2) {
		t.Error("UpdatedAt should be updated after health update")
	}
}

func TestMetadataUpdate(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	stateBefore := inst.State

	t1 := now.Add(time.Second)
	err := inst.SetMetadata("host", "node-01", t1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.Metadata["host"] != "node-01" {
		t.Errorf("expected metadata host=node-01, got %s", inst.Metadata["host"])
	}
	if inst.State != stateBefore {
		t.Error("metadata update should not change Runtime State")
	}
	if !inst.UpdatedAt.Equal(t1) {
		t.Error("UpdatedAt should be updated after metadata update")
	}
}

func TestSetMetadataRejectsEmptyKey(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	err := inst.SetMetadata("", "value", now)
	if err == nil {
		t.Error("expected error for empty key")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestSetMetadataRejectsControlCharacters(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	err := inst.SetMetadata("bad\x00key", "value", now)
	if err == nil {
		t.Error("expected error for control character in key")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestTransitionFromTerminalState(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	t1 := now.Add(time.Second)
	inst.Transition(RuntimeStateStarting, "begin", t1)
	inst.Transition(RuntimeStateFailed, "crash", t1)

	err := inst.Transition(RuntimeStateRunning, "try_recover", now.Add(2*time.Second))
	if err == nil {
		t.Error("expected error when transitioning from terminal state")
	}
	if !IsHostError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state error, got %v", err)
	}
}

func TestTransitionToSameState(t *testing.T) {
	now := time.Now()
	inst, _ := NewRuntimeInstance("rt-001", "plugin-a", now)

	err := inst.Transition(RuntimeStateCreated, "no_op", now.Add(time.Second))
	if err == nil {
		t.Error("expected error when transitioning to same state")
	}
	if !IsHostError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state error, got %v", err)
	}
}

func TestTerminalStateCheck(t *testing.T) {
	if !IsTerminalRuntimeState(RuntimeStateStopped) {
		t.Error("stopped should be terminal")
	}
	if !IsTerminalRuntimeState(RuntimeStateFailed) {
		t.Error("failed should be terminal")
	}
	if IsTerminalRuntimeState(RuntimeStateRunning) {
		t.Error("running should not be terminal")
	}
}

func TestUnknownRuntimeStateInvalid(t *testing.T) {
	if IsValidRuntimeState(RuntimeState("banana")) {
		t.Error("banana should be invalid")
	}
}
