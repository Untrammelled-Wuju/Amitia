// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/lifecycle"
)

type mockSandboxInspector struct {
	snap SandboxRecoverySnapshot
}

func (m *mockSandboxInspector) RecoverySnapshot(_ context.Context) SandboxRecoverySnapshot {
	return m.snap
}

func TestBuildIOSSandboxRecoveryScanHook_NilInspector(t *testing.T) {
	ctx := SandboxScanContext{
		ProviderEnabled: true,
		Inspector:       nil,
	}
	hook := BuildIOSSandboxRecoveryScanHook(ctx)

	items, err := hook(context.Background(), "test-startup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestBuildIOSSandboxRecoveryScanHook_RuntimeRecovery(t *testing.T) {
	ctx := SandboxScanContext{
		ProviderEnabled: true,
		Inspector: &mockSandboxInspector{
			snap: SandboxRecoverySnapshot{
				RuntimeID:           "runtime-1",
				LifecycleState:      SandboxStateFailed,
				Generation:          4,
				DesiredRunning:      true,
				RecoveryPending:     true,
				RootfsInstalled:     true,
				ActiveRootfsVersion: "3.19",
			},
		},
		NowFn: func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) },
	}
	hook := BuildIOSSandboxRecoveryScanHook(ctx)

	items, err := hook(context.Background(), "startup-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected at least one recovery item")
	}

	found := false
	for _, item := range items {
		if item.Action == "recover_ios_sandbox" {
			found = true
			if item.Severity != "high" {
				t.Errorf("expected high severity, got %s", item.Severity)
			}
			if item.ComponentID != "provider.ios-sandbox" {
				t.Errorf("expected provider.ios-sandbox, got %s", item.ComponentID)
			}
		}
	}

	if !found {
		t.Error("expected recover_ios_sandbox action")
	}
}

func TestBuildIOSSandboxRecoveryScanHook_RootfsRepair(t *testing.T) {
	ctx := SandboxScanContext{
		ProviderEnabled: true,
		Inspector: &mockSandboxInspector{
			snap: SandboxRecoverySnapshot{
				RuntimeID:       "runtime-1",
				LifecycleState:  SandboxStateFailed,
				Generation:      4,
				DesiredRunning:  true,
				RootfsCorrupted: true,
			},
		},
	}
	hook := BuildIOSSandboxRecoveryScanHook(ctx)

	items, err := hook(context.Background(), "startup-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, item := range items {
		if item.Action == "repair_rootfs" {
			found = true
			if item.Severity != "critical" {
				t.Errorf("expected critical severity, got %s", item.Severity)
			}
		}
	}

	if !found {
		t.Error("expected repair_rootfs action")
	}
}

func TestBuildIOSSandboxRecoveryScanHook_DiscardInterrupted(t *testing.T) {
	ctx := SandboxScanContext{
		ProviderEnabled: true,
		Inspector: &mockSandboxInspector{
			snap: SandboxRecoverySnapshot{
				RuntimeID:         "runtime-1",
				LifecycleState:    SandboxStateFailed,
				Generation:        4,
				DesiredRunning:    true,
				ActiveExecutionID: "exec-123",
			},
		},
	}
	hook := BuildIOSSandboxRecoveryScanHook(ctx)

	items, err := hook(context.Background(), "startup-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDiscard := false
	for _, item := range items {
		if item.Action == "discard_interrupted_execution" {
			foundDiscard = true
			if item.Severity != "warning" {
				t.Errorf("expected warning severity, got %s", item.Severity)
			}
		}
	}

	if !foundDiscard {
		t.Error("expected discard_interrupted_execution action")
	}
}

func TestBuildIOSSandboxRecoveryScanHook_DisabledProvider(t *testing.T) {
	ctx := SandboxScanContext{
		ProviderEnabled: false,
		Inspector: &mockSandboxInspector{
			snap: SandboxRecoverySnapshot{
				RuntimeID:      "runtime-1",
				LifecycleState: SandboxStateFailed,
				DesiredRunning: false,
			},
		},
	}
	hook := BuildIOSSandboxRecoveryScanHook(ctx)

	items, err := hook(context.Background(), "startup-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items when provider disabled and not desired, got %d", len(items))
	}
}

func TestBuildIOSSandboxRecoveryScanHook_CallsRealScan(t *testing.T) {
	ctx := SandboxScanContext{
		ProviderEnabled: true,
		Inspector: &mockSandboxInspector{
			snap: SandboxRecoverySnapshot{
				RuntimeID:           "runtime-1",
				LifecycleState:      SandboxStateRecoveryPending,
				Generation:          5,
				DesiredRunning:      true,
				ActiveRootfsVersion: "3.19",
				RunningRootfsVersion: "3.18",
				RestartRequired:     true,
			},
		},
	}
	hook := BuildIOSSandboxRecoveryScanHook(ctx)

	report := &lifecycle.RecoveryReport{
		StartupID: "startup-123",
	}
	_ = report

	items, err := hook(context.Background(), "startup-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected recovery items for restart_with_active_rootfs scenario")
	}
}
