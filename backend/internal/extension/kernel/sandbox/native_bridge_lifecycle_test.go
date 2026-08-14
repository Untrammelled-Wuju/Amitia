// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build !ios
// +build !ios

package sandbox

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLifecycleState_InitialState(t *testing.T) {
	client := newIOSNativeBridgeClient()
	state := client.LifecycleState(context.Background())
	if state != SandboxStateIdle {
		t.Errorf("expected idle, got %s", state)
	}
}

func TestLifecycleState_Transitions_ConcurrentAccess(t *testing.T) {
	client := newIOSNativeBridgeClient()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.LifecycleState(ctx)
			_ = client.Health(ctx)
		}()
	}
	wg.Wait()
}

func TestCanTransitionSandboxState_ConcurrencySafe(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			CanTransitionSandboxState(SandboxStateIdle, SandboxStateStarting)
			CanTransitionSandboxState(SandboxStateRunning, SandboxStateQuiescing)
			CanTransitionSandboxState(SandboxStateFailed, SandboxStateRecoveryPending)
		}()
	}
	wg.Wait()
}

func TestSandboxStopReason_Values(t *testing.T) {
	reasons := []SandboxStopReason{
		StopReasonUser,
		StopReasonApplicationShutdown,
		StopReasonRestart,
		StopReasonRecovery,
		StopReasonDisable,
	}

	for _, r := range reasons {
		if string(r) == "" {
			t.Error("stop reason should not be empty")
		}
	}
}

func TestSandboxRestartReason_Values(t *testing.T) {
	reasons := []SandboxRestartReason{
		RestartReasonRootfsChanged,
		RestartReasonConfigurationChanged,
		RestartReasonHealthFailure,
		RestartReasonManual,
		RestartReasonRecovery,
	}

	for _, r := range reasons {
		if string(r) == "" {
			t.Error("restart reason should not be empty")
		}
	}
}

func TestSandboxRecoveryDescriptor_Fields(t *testing.T) {
	now := time.Now().UTC()
	desc := SandboxRecoveryDescriptor{
		SchemaVersion:           1,
		RuntimeID:               "runtime-1",
		DesiredRunning:          true,
		Generation:              7,
		ActiveRootfsVersion:     "3.19",
		ActiveRootfsDigest:      "abc123",
		LastKnownLifecycleState: "running",
		CleanShutdown:           true,
		Timestamp:               now,
	}

	if desc.Generation != 7 {
		t.Errorf("expected generation 7, got %d", desc.Generation)
	}
	if !desc.DesiredRunning {
		t.Error("expected desired running true")
	}
	if desc.LastKnownLifecycleState != "running" {
		t.Errorf("expected running state, got %s", desc.LastKnownLifecycleState)
	}
}

func TestIOSNocgoClient_LifecycleState(t *testing.T) {
	client := newIOSNativeBridgeClient()

	state := client.LifecycleState(context.Background())
	if state != SandboxStateIdle {
		t.Errorf("expected idle state, got %s", state)
	}

	snap := client.RecoverySnapshot(context.Background())
	if snap.LifecycleState != SandboxStateIdle {
		t.Errorf("expected idle in snapshot, got %s", snap.LifecycleState)
	}
}

func TestIOSNocgoClient_QuiesceResume(t *testing.T) {
	client := newIOSNativeBridgeClient()
	ctx := context.Background()

	err := client.Quiesce(ctx)
	if err == nil {
		t.Error("expected error when quiescing in idle state")
	}

	le, ok := err.(*SandboxLifecycleError)
	if !ok {
		t.Errorf("expected SandboxLifecycleError, got %T", err)
	} else if le.State != SandboxStateIdle {
		t.Errorf("expected idle state in error, got %s", le.State)
	}

	err = client.Resume(ctx)
	if err == nil {
		t.Error("expected error when resuming in idle state")
	}
}

func TestIOSNocgoClient_RestartRecover(t *testing.T) {
	client := newIOSNativeBridgeClient()
	ctx := context.Background()

	err := client.Restart(ctx, RestartReasonManual)
	if err == nil {
		t.Error("expected error when restarting in idle state")
	}

	err = client.Recover(ctx)
	if err == nil {
		t.Error("expected error when recovering in idle state")
	}
}
