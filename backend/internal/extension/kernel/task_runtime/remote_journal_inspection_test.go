package task_runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type testJournalInvocationPort struct {
	calls int
	reply capability.UnifiedToolResult
	input json.RawMessage
}

func (p *testJournalInvocationPort) Execute(_ context.Context, req capability.DeviceRuntimeInvocationRequest) capability.UnifiedToolResult {
	p.calls++
	p.input = append([]byte(nil), req.Input...)
	if req.Route.Binding.HandlerName != "task.journal.inspect" {
		panic("wrong handler")
	}
	return p.reply
}
func (p *testJournalInvocationPort) Health(context.Context, capability.RuntimeExecutionRoute) capability.HealthStatus {
	return capability.HealthUnknown
}
func (p *testJournalInvocationPort) Cancel(context.Context, capability.DeviceRuntimeInvocationRequest, capability.ToolCancellationReason) error {
	return nil
}

func TestRemoteJournalInspectionRequiresOwnerAndDoesNotResume(t *testing.T) {
	run := &TaskRun{TaskRunID: "t-1", TaskDefinitionID: "task.action", Generation: 2, ScopeSnapshotID: "scope-1", LeaseID: "lease-1", ExecutionAttemptID: "attempt-1", ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 5}}
	scope := coordination.ExecutionScope{CoreID: "core", SpaceID: "core", ResourceOwnerID: "core", TargetDeviceID: "device", Coordinated: true}
	ctx := coordination.WithScope(t.Context(), scope)
	port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess, DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "new-session", Generation: 2, Structured: json.RawMessage(`{"state":"performed"}`)}}
	executor := NewMeshRemoteTaskExecutor(nil, nil)
	executor.SetInspectionPort(port)
	if _, err := executor.InspectExecutionJournal(t.Context(), run); err == nil || port.calls != 0 {
		t.Fatalf("without owner: calls=%d err=%v", port.calls, err)
	}
	got, err := executor.InspectExecutionJournal(ctx, run)
	if err != nil || got.State != "performed" || got.Result != nil || port.calls != 1 {
		t.Fatalf("inspection %+v calls=%d err=%v", got, port.calls, err)
	}
	var req struct {
		TaskRunID            string `json:"taskRunId"`
		TaskGeneration       int64  `json:"taskGeneration"`
		AttemptID            string `json:"attemptId"`
		LeaseID              string `json:"leaseId"`
		RuntimeSessionID     string `json:"runtimeSessionId"`
		ConnectionGeneration int64  `json:"connectionGeneration"`
		TaskDefinitionID     string `json:"taskDefinitionId"`
	}
	if json.Unmarshal(port.input, &req) != nil || req.TaskRunID != "t-1" || req.TaskGeneration != 2 || req.AttemptID != "attempt-1" || req.LeaseID != "lease-1" || req.RuntimeSessionID != "old-session" || req.ConnectionGeneration != 5 || req.TaskDefinitionID != "task.action" {
		t.Fatalf("mismatched inspection identity: %s", port.input)
	}
	port.reply.Structured = json.RawMessage(`{"state":"performed","result":{"status":"completed"}}`)
	if _, err := executor.InspectExecutionJournal(ctx, run); err == nil {
		t.Fatal("device-private result accepted")
	}
	port.reply.Structured = json.RawMessage(`{"state":"invented"}`)
	if _, err := executor.InspectExecutionJournal(ctx, run); err == nil {
		t.Fatal("invented inspection status accepted")
	}
	port.reply.Structured = json.RawMessage(`{"state":"performed"}`)
	port.reply.DeviceID = "different-device"
	if _, err := executor.InspectExecutionJournal(ctx, run); err == nil {
		t.Fatal("different device result accepted")
	}
}

func TestRemoteJournalInspectionUsesSourceTaskIDNotCoreCatalogAlias(t *testing.T) {
	run := &TaskRun{TaskRunID: "alias-run", TaskDefinitionID: "mesh-task-catalog", Generation: 3, ScopeSnapshotID: "scope", LeaseID: "lease", ExecutionAttemptID: "attempt", ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 7, SourceTaskDefinitionID: "real-device-task"}}
	scope := coordination.ExecutionScope{CoreID: "core", SpaceID: "core", ResourceOwnerID: "core", TargetDeviceID: "device", Coordinated: true}
	port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess, DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "new-session", Generation: 8, Structured: json.RawMessage(`{"state":"started"}`)}}
	executor := NewMeshRemoteTaskExecutor(nil, nil)
	executor.SetInspectionPort(port)
	if _, err := executor.InspectExecutionJournal(coordination.WithScope(t.Context(), scope), run); err != nil {
		t.Fatal(err)
	}
	var request struct {
		TaskDefinitionID string `json:"taskDefinitionId"`
	}
	if json.Unmarshal(port.input, &request) != nil || request.TaskDefinitionID != "real-device-task" {
		t.Fatalf("alias leaked into device identity: %s", port.input)
	}
}

func TestRemoteJournalInspectionStopReceiptIsReadOnlyEvidence(t *testing.T) {
	run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task.action", Generation: 3, ScopeSnapshotID: "scope", LeaseID: "lease", ExecutionAttemptID: "attempt", ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 7}}
	owner := coordination.ExecutionScope{CoreID: "core", SpaceID: "core", ResourceOwnerID: "core", TargetDeviceID: "device", Coordinated: true}
	ctx := coordination.WithScope(t.Context(), owner)
	port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess, DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "current-session", Generation: 8, Structured: json.RawMessage(`{"state":"stopped_performed"}`)}}
	executor := NewMeshRemoteTaskExecutor(nil, nil)
	executor.SetInspectionPort(port)
	for _, state := range []string{"stopped_performed", "stopped_unknown"} {
		port.reply.Structured = json.RawMessage(`{"state":"` + state + `"}`)
		got, err := executor.InspectExecutionJournal(ctx, run)
		if err != nil || got.State != state || got.Result != nil {
			t.Fatalf("state %s: %+v %v", state, got, err)
		}
	}
	port.reply.Structured = json.RawMessage(`{"state":"stopped_performed","result":{"status":"completed"}}`)
	if _, err := executor.InspectExecutionJournal(ctx, run); err == nil {
		t.Fatal("Owner-private result smuggled in stop receipt")
	}
}

func TestPausedRemoteCanUsePersistedExactStopAfterCoreRestart(t *testing.T) {
	at := time.Now().UTC()
	checkpoint := "owner-confirmed-checkpoint"
	run := &TaskRun{TaskRunID: "resume", TaskDefinitionID: "task.action", Generation: 4,
		Status: RunStatusPaused, PausedAt: &at, CheckpointID: &checkpoint, ScopeSnapshotID: "scope", LeaseID: "lease",
		ExecutionAttemptID: "attempt", ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 5}}
	owner := coordination.ExecutionScope{CoreID: "core", SpaceID: "core", ResourceOwnerID: "core", TargetDeviceID: "device", Coordinated: true}
	ctx := coordination.WithScope(t.Context(), owner)
	port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess, DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "new-session", Generation: 6, Structured: json.RawMessage(`{"state":"stopped_performed"}`)}}

	executor := NewMeshRemoteTaskExecutor(nil, nil)
	executor.SetInspectionPort(port)
	if !executor.ConfirmPauseStopped(ctx, run) || port.calls != 1 {
		t.Fatalf("verified durable stop not accepted: calls=%d", port.calls)
	}
	for _, state := range []string{"started", "performed", "stopped_unknown", "absent"} {
		port.reply.Structured = json.RawMessage(`{"state":"` + state + `"}`)
		if executor.ConfirmPauseStopped(ctx, run) {
			t.Fatalf("unsafe %s upgraded to stop evidence", state)
		}
	}
	port.reply.Structured = json.RawMessage(`{"state":"stopped_performed"}`)
	withoutCheckpoint := *run
	withoutCheckpoint.CheckpointID = nil
	if executor.ConfirmPauseStopped(ctx, &withoutCheckpoint) {
		t.Fatal("pause without Owner checkpoint resumed")
	}
	if executor.ConfirmPauseStopped(t.Context(), run) {
		t.Fatal("unowned confirmation accepted")
	}
	var request struct {
		AttemptID string `json:"attemptId"`
	}
	if json.Unmarshal(port.input, &request) != nil || request.AttemptID != "attempt" {
		t.Fatalf("pause recovery did not bind original attempt: %s", port.input)
	}
}
