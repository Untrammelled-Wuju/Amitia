// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"testing"
)

func TestCanTransitionSandboxState_ValidTransitions(t *testing.T) {
	valid := []struct {
		from, to SandboxLifecycleState
	}{
		{SandboxStateIdle, SandboxStateStarting},
		{SandboxStateStarting, SandboxStateRunning},
		{SandboxStateStarting, SandboxStateFailed},
		{SandboxStateStarting, SandboxStateStopping},
		{SandboxStateRunning, SandboxStateQuiescing},
		{SandboxStateRunning, SandboxStateStopping},
		{SandboxStateRunning, SandboxStateFailed},
		{SandboxStateQuiescing, SandboxStateQuiesced},
		{SandboxStateQuiescing, SandboxStateStopping},
		{SandboxStateQuiescing, SandboxStateFailed},
		{SandboxStateQuiesced, SandboxStateRunning},
		{SandboxStateQuiesced, SandboxStateStopping},
		{SandboxStateQuiesced, SandboxStateFailed},
		{SandboxStateFailed, SandboxStateRecoveryPending},
		{SandboxStateFailed, SandboxStateStopping},
		{SandboxStateRecoveryPending, SandboxStateRecovering},
		{SandboxStateRecoveryPending, SandboxStateStopping},
		{SandboxStateRecovering, SandboxStateRunning},
		{SandboxStateRecovering, SandboxStateFailed},
		{SandboxStateRecovering, SandboxStateStopping},
		{SandboxStateStopping, SandboxStateIdle},
		{SandboxStateStopping, SandboxStateFailed},
	}

	for _, tc := range valid {
		if !CanTransitionSandboxState(tc.from, tc.to) {
			t.Errorf("expected valid transition: %s -> %s", tc.from, tc.to)
		}
	}
}

func TestCanTransitionSandboxState_InvalidTransitions(t *testing.T) {
	invalid := []struct {
		from, to SandboxLifecycleState
	}{
		{SandboxStateIdle, SandboxStateRunning},
		{SandboxStateIdle, SandboxStateFailed},
		{SandboxStateIdle, SandboxStateStopping},
		{SandboxStateStarting, SandboxStateQuiesced},
		{SandboxStateFailed, SandboxStateRunning},
		{SandboxStateFailed, SandboxStateIdle},
		{SandboxStateQuiesced, SandboxStateIdle},
		{SandboxStateStopping, SandboxStateRunning},
		{SandboxStateRecoveryPending, SandboxStateRunning},
		{SandboxStateRunning, SandboxStateIdle},
	}

	for _, tc := range invalid {
		if CanTransitionSandboxState(tc.from, tc.to) {
			t.Errorf("expected invalid transition: %s -> %s", tc.from, tc.to)
		}
	}
}

func TestCanTransitionSandboxState_SameState(t *testing.T) {
	states := []SandboxLifecycleState{
		SandboxStateIdle,
		SandboxStateStarting,
		SandboxStateRunning,
		SandboxStateQuiescing,
		SandboxStateQuiesced,
		SandboxStateStopping,
		SandboxStateFailed,
		SandboxStateRecoveryPending,
		SandboxStateRecovering,
	}

	for _, s := range states {
		if CanTransitionSandboxState(s, s) {
			t.Errorf("expected same-state transition to be invalid: %s", s)
		}
	}
}

func TestIsValidSandboxState(t *testing.T) {
	validStates := []SandboxLifecycleState{
		SandboxStateIdle,
		SandboxStateStarting,
		SandboxStateRunning,
		SandboxStateQuiescing,
		SandboxStateQuiesced,
		SandboxStateStopping,
		SandboxStateFailed,
		SandboxStateRecoveryPending,
		SandboxStateRecovering,
	}

	for _, s := range validStates {
		if !IsValidSandboxState(s) {
			t.Errorf("expected valid state: %s", s)
		}
	}

	invalidStates := []SandboxLifecycleState{
		SandboxLifecycleState(""),
		SandboxLifecycleState("invalid"),
		SandboxLifecycleState("STARTING"),
		SandboxLifecycleState("RUNNING"),
	}

	for _, s := range invalidStates {
		if IsValidSandboxState(s) {
			t.Errorf("expected invalid state: %s", s)
		}
	}
}

func TestCanExecuteInState(t *testing.T) {
	if !CanExecuteInState(SandboxStateRunning) {
		t.Error("expected running state to allow execution")
	}

	nonRunningStates := []SandboxLifecycleState{
		SandboxStateIdle,
		SandboxStateStarting,
		SandboxStateQuiescing,
		SandboxStateQuiesced,
		SandboxStateStopping,
		SandboxStateFailed,
		SandboxStateRecoveryPending,
		SandboxStateRecovering,
	}

	for _, s := range nonRunningStates {
		if CanExecuteInState(s) {
			t.Errorf("expected state %s to reject execution", s)
		}
	}
}

func TestSandboxLifecycleError(t *testing.T) {
	err := &SandboxLifecycleError{
		Code:  SandboxErrInvalidState,
		State: SandboxStateIdle,
	}
	expected := "SANDBOX_INVALID_STATE (state=idle)"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestSandboxLifecycleError_WithCause(t *testing.T) {
	cause := &NativeBridgeError{Code: NativeErrRuntimeNotStarted, Message: "not ready"}
	err := &SandboxLifecycleError{
		Code:  SandboxErrStartCancelled,
		State: SandboxStateStarting,
		Cause: cause,
	}
	if err.Unwrap() != cause {
		t.Error("expected cause to be unwrapable")
	}
}
