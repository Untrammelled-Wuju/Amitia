package system

import (
	"testing"

	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

func TestRuntimeSnapshotUnavailableIgnoresOptionalDegradation(t *testing.T) {
	optional := runtimeorchestrator.RuntimeSnapshot{
		State:         runtimeorchestrator.OrchestratorDegraded,
		DegradedCount: 1,
	}
	if runtimeSnapshotUnavailable(optional) {
		t.Fatal("optional degradation must remain available")
	}

	required := runtimeorchestrator.RuntimeSnapshot{
		State:         runtimeorchestrator.OrchestratorDegraded,
		DegradedCount: 1,
		BlockingCount: 1,
	}
	if !runtimeSnapshotUnavailable(required) {
		t.Fatal("blocking degradation must be unavailable")
	}

	failed := runtimeorchestrator.RuntimeSnapshot{
		State:       runtimeorchestrator.OrchestratorReady,
		FailedCount: 1,
	}
	if !runtimeSnapshotUnavailable(failed) {
		t.Fatal("failed component must be unavailable")
	}
}
