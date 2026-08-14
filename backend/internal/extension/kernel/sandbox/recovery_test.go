// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"testing"
	"time"
)

func TestSandboxRecoverySnapshot_Classify(t *testing.T) {
	policy := DefaultRecoveryPolicy

	tests := []struct {
		name     string
		snap     SandboxRecoverySnapshot
		enabled  bool
		expected SandboxRecoveryClass
	}{
		{
			name: "disabled provider returns disabled",
			snap: SandboxRecoverySnapshot{
				DesiredRunning: true,
				RuntimeID:      "runtime-1",
			},
			enabled:  false,
			expected: SandboxRecoveryDisabled,
		},
		{
			name: "rootfs corrupted returns manual repair",
			snap: SandboxRecoverySnapshot{
				DesiredRunning:  true,
				RuntimeID:       "runtime-1",
				RootfsCorrupted: true,
			},
			enabled:  true,
			expected: SandboxRecoveryManualRepair,
		},
		{
			name: "desired running with restart rootfs change",
			snap: SandboxRecoverySnapshot{
				DesiredRunning:       true,
				RuntimeID:            "runtime-1",
				RestartRequired:      true,
				ActiveRootfsVersion:  "3.19",
				RunningRootfsVersion: "3.18",
			},
			enabled:  true,
			expected: SandboxRecoveryRestartWithActiveRootfs,
		},
		{
			name: "desired running restart runtime",
			snap: SandboxRecoverySnapshot{
				DesiredRunning:       true,
				RuntimeID:            "runtime-1",
				ActiveRootfsVersion:  "3.19",
				RunningRootfsVersion: "3.19",
			},
			enabled:  true,
			expected: SandboxRecoveryRestartRuntime,
		},
		{
			name: "not desired running returns none",
			snap: SandboxRecoverySnapshot{
				DesiredRunning: false,
				RuntimeID:      "runtime-1",
			},
			enabled:  true,
			expected: SandboxRecoveryNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.snap.Classify(policy, tc.enabled)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestBuildRecoveryDescriptor(t *testing.T) {
	snap := SandboxRecoverySnapshot{
		RuntimeID:            "runtime-1",
		LifecycleState:       SandboxStateFailed,
		Generation:           7,
		DesiredRunning:       true,
		RecoveryPending:      true,
		ActiveRootfsVersion:  "3.19",
		ActiveRootfsDigest:   "abc123",
		RunningRootfsVersion: "3.18",
	}

	desc := BuildRecoveryDescriptor(true, snap, false)

	if desc.RuntimeID != "runtime-1" {
		t.Errorf("expected runtime-1, got %s", desc.RuntimeID)
	}
	if desc.Generation != 7 {
		t.Errorf("expected generation 7, got %d", desc.Generation)
	}
	if !desc.DesiredRunning {
		t.Error("expected desired running to be true")
	}
	if desc.CleanShutdown {
		t.Error("expected clean shutdown to be false")
	}
	if desc.LastKnownLifecycleState != "failed" {
		t.Errorf("expected failed, got %s", desc.LastKnownLifecycleState)
	}
}

func TestBuildRecoveryDescriptor_DisabledProvider(t *testing.T) {
	snap := SandboxRecoverySnapshot{
		RuntimeID:      "runtime-1",
		Generation:     5,
		DesiredRunning: true,
	}

	desc := BuildRecoveryDescriptor(false, snap, true)
	if desc.DesiredRunning {
		t.Error("expected desired running to be false when provider disabled")
	}
	if !desc.CleanShutdown {
		t.Error("expected clean shutdown to be true")
	}
}

func TestDefaultRecoveryPolicy(t *testing.T) {
	if DefaultRecoveryPolicy.MaxAttempts != 3 {
		t.Errorf("expected 3 max attempts, got %d", DefaultRecoveryPolicy.MaxAttempts)
	}
	if DefaultRecoveryPolicy.InitialBackoff != 500*time.Millisecond {
		t.Errorf("expected 500ms initial backoff, got %v", DefaultRecoveryPolicy.InitialBackoff)
	}
	if DefaultRecoveryPolicy.MaxBackoff != 30*time.Second {
		t.Errorf("expected 30s max backoff, got %v", DefaultRecoveryPolicy.MaxBackoff)
	}
}
