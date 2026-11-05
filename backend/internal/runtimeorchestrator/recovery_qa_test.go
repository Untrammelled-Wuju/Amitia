package runtimeorchestrator

import (
	"errors"
	"testing"
)

func TestHealthRefreshRecoversAggregateAfterTransientRequiredFailureQA(t *testing.T) {
	orch := newTestOrchestrator()
	component := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.required", Phase: PhaseInfrastructure, Enabled: true, Required: true,
	}}
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 3; round++ {
		failure := errors.New("QA-required-health-transient")
		component.readyErr = failure
		failed := orch.RefreshHealth(t.Context())
		if failed.State != OrchestratorBlocked || failed.BlockingCount != 1 || failed.Components[component.ID].LastError != failure.Error() {
			t.Fatalf("round %d lost current failure: %+v", round, failed)
		}
		component.readyErr = nil
		recovered := orch.RefreshHealth(t.Context())
		if recovered.State != OrchestratorReady || !recovered.IsReady() || recovered.IsBlocked() || recovered.BlockingCount != 0 || recovered.FailedCount != 0 {
			t.Fatalf("round %d aggregate failed to recover with healthy component: %+v", round, recovered)
		}
		if component.startCalls != 1 || component.stopCalls != 0 {
			t.Fatalf("health recovery replayed lifecycle: start=%d stop=%d", component.startCalls, component.stopCalls)
		}
	}
}

func TestReportedRequiredFailureBlocksUntilRecoveryQA(t *testing.T) {
	orch := newTestOrchestrator()
	component := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.reported", Phase: PhaseInfrastructure, Enabled: true, Required: true,
	}}
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}
	if err := orch.ReportComponentState(component.ID, StateFailed, errors.New("QA-reported-failure")); err != nil {
		t.Fatal(err)
	}
	if snapshot := orch.Snapshot(); snapshot.State != OrchestratorBlocked || snapshot.BlockingCount != 1 {
		t.Fatalf("reported required failure did not block aggregate: %+v", snapshot)
	}
}
