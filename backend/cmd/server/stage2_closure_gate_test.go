package main

import (
	"testing"
)

func TestStage2ArchitectureReadyServicesNil(t *testing.T) {
	gate := NewStage2ClosureGateAdapter(nil)
	result := gate.ArchitectureReady()
	if result.Ready {
		t.Fatal("expected not ready when services is nil")
	}
}

func TestStage2ArchitectureReadyKernelContainerNil(t *testing.T) {
	gate := NewStage2ClosureGateAdapter(&AppServices{})
	result := gate.ArchitectureReady()
	if result.Ready {
		t.Fatal("expected not ready when no components wired")
	}
	if len(result.Reasons) == 0 {
		t.Fatal("expected failure reasons to be reported")
	}
}

func TestStage2CanRunCutoverReportsReasons(t *testing.T) {
	gate := NewStage2ClosureGateAdapter(&AppServices{})
	canRun, reasons := gate.CanRunCutover()
	if canRun {
		t.Fatal("expected cannot cutover when architecture not ready")
	}
	if len(reasons) == 0 {
		t.Fatal("expected failure reasons when architecture incomplete")
	}

	msg := gate.FailureMessage(reasons)
	if msg == "" {
		t.Fatal("expected non-empty failure message")
	}
}

func TestStage2AssertFreshInstallCannotBypassCutover(t *testing.T) {
	if err := assertFreshInstallCannotBypassCutover(true, false); err == nil {
		t.Fatal("expected fresh install to be blocked when closure is not ready")
	}
	if err := assertFreshInstallCannotBypassCutover(true, true); err != nil {
		t.Fatalf("fresh install with verified closure should be allowed: %v", err)
	}
	if err := assertFreshInstallCannotBypassCutover(false, false); err != nil {
		t.Fatalf("non-fresh install is governed by the normal closure path: %v", err)
	}
}
