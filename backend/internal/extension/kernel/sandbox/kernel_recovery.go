// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/lifecycle"
)

type SandboxScanContext struct {
	ProviderEnabled bool
	Inspector       SandboxRecoveryInspector
	NowFn           func() time.Time
}

func BuildIOSSandboxRecoveryScanHook(ctx SandboxScanContext) lifecycle.ScanHook {
	nowFn := ctx.NowFn
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}

	return func(scanCtx context.Context, startupID string) ([]lifecycle.RecoveryItem, error) {
		if ctx.Inspector == nil {
			return nil, nil
		}

		snap := ctx.Inspector.RecoverySnapshot(scanCtx)
		items := evaluateSandboxRecoveryItems(snap, ctx.ProviderEnabled, startupID, nowFn())
		return items, nil
	}
}

func evaluateSandboxRecoveryItems(snap SandboxRecoverySnapshot, providerEnabled bool, startupID string, now time.Time) []lifecycle.RecoveryItem {
	var items []lifecycle.RecoveryItem

	if !providerEnabled && !snap.DesiredRunning {
		return nil
	}

	class := snap.Classify(DefaultRecoveryPolicy, providerEnabled)

	switch class {
	case SandboxRecoveryRestartRuntime, SandboxRecoveryRestartWithActiveRootfs:
		items = append(items, lifecycle.RecoveryItem{
			Category:    "runtime",
			ComponentID: "provider.ios-sandbox",
			Subject:     "ios-sandbox-recovery",
			Severity:    "high",
			Action:      "recover_ios_sandbox",
			Metadata: map[string]any{
				"startupId":      startupID,
				"generation":     snap.Generation,
				"desiredRunning": snap.DesiredRunning,
				"lastState":      string(snap.LifecycleState),
				"scheduledAt":    now.Format(time.RFC3339),
			},
		})
	case SandboxRecoveryManualRepair:
		items = append(items, lifecycle.RecoveryItem{
			Category:    "runtime",
			ComponentID: "provider.ios-sandbox",
			Subject:     "ios-sandbox-rootfs-repair",
			Severity:    "critical",
			Action:      "repair_rootfs",
			Metadata: map[string]any{
				"startupId":   startupID,
				"generation":  snap.Generation,
				"scheduledAt": now.Format(time.RFC3339),
			},
		})
	}

	if snap.ActiveExecutionID != "" && (snap.LifecycleState == SandboxStateFailed || snap.LifecycleState == SandboxStateRecoveryPending || snap.LifecycleState == SandboxStateRecovering) {
		items = append(items, lifecycle.RecoveryItem{
			Category:    "runtime",
			ComponentID: "provider.ios-sandbox",
			Subject:     "ios-sandbox-discard-interrupted",
			Severity:    "warning",
			Action:      "discard_interrupted_execution",
			Metadata: map[string]any{
				"startupId":   startupID,
				"scheduledAt": now.Format(time.RFC3339),
			},
		})
	}

	return items
}
