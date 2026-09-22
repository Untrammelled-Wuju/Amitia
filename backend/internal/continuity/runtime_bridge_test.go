package continuity

import (
	"context"
	"testing"
	"time"
)

func TestRuntimeBridgeWorkflowCreatesAndCompletesWaits(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "工作流持续事项", Status: ThreadStatusActive}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	if err := repo.Bind(thread.ID, "execution", "run-1", "execution", "test", 1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Bind(thread.ID, "conversation", "conv-1", "context", "test", 1); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	bridge := NewRuntimeBridge(repo, coordinator)
	now := time.Now().UTC()
	bridge.OnWorkflow(context.Background(), WorkflowLifecycleSignal{
		Type: "waiting_device", WorkflowID: "wf-1", ExecutionID: "run-1", SpaceID: "space-1",
		DeviceID: "device-1", Status: "waiting_device", Timestamp: now,
	})
	waits, err := repo.ListOpenWaits(thread.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(waits) != 1 || waits[0].WaitType != WaitTypeDevice {
		t.Fatalf("waits=%#v, want one device wait", waits)
	}
	updated, _ := repo.GetThread(thread.ID, "space-1")
	if updated.Status != ThreadStatusWaiting {
		t.Fatalf("thread status=%s, want waiting", updated.Status)
	}
	if _, err := coordinator.CancelWait(context.Background(), waits[0].ID, "test"); err != nil {
		t.Fatal(err)
	}
	updated, _ = repo.GetThread(thread.ID, "space-1")
	if _, err := repo.UpdateThreadCAS(thread.ID, updated.Revision, map[string]interface{}{"status": ThreadStatusWaiting}); err != nil {
		t.Fatal(err)
	}
	dependency := &Wait{ThreadID: thread.ID, WaitType: WaitTypeDependency, Status: WaitStatusWaiting, ConditionJSON: `{"workflowRunId":"run-1"}`, AutoResume: true}
	if err := repo.CreateWait(dependency); err != nil {
		t.Fatal(err)
	}
	bridge.OnWorkflow(context.Background(), WorkflowLifecycleSignal{
		Type: "completed", WorkflowID: "wf-1", ExecutionID: "run-1", SpaceID: "space-1",
		Status: "succeeded", Timestamp: now.Add(time.Second),
	})
	resolved, _ := repo.GetWait(dependency.ID)
	if resolved.Status != WaitStatusResolved || resolved.WakeState != WakeStatePending {
		t.Fatalf("dependency wait status=%s wake=%s", resolved.Status, resolved.WakeState)
	}
	updated, _ = repo.GetThread(thread.ID, "space-1")
	if updated.CurrentState != "工作流执行已完成" {
		t.Fatalf("current state=%q", updated.CurrentState)
	}
}

func TestRuntimeBridgeTaskCompletedResolvesDependency(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "任务持续事项", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	if err := repo.Bind(thread.ID, "task_run", "task-1", "execution", "test", 1); err != nil {
		t.Fatal(err)
	}
	dependency := &Wait{ThreadID: thread.ID, WaitType: WaitTypeDependency, Status: WaitStatusWaiting, ConditionJSON: `{"taskRunId":"task-1"}`, AutoResume: true}
	if err := repo.CreateWait(dependency); err != nil {
		t.Fatal(err)
	}
	coordinator := NewWaitCoordinator(repo, &recordingWakeDispatcher{}, DefaultWaitCoordinatorConfig())
	bridge := NewRuntimeBridge(repo, coordinator)
	bridge.OnTask(context.Background(), TaskLifecycleSignal{
		Type: "terminal", TaskRunID: "task-1", SpaceID: "space-1", Status: "succeeded", Timestamp: time.Now().UTC(),
	})
	resolved, _ := repo.GetWait(dependency.ID)
	if resolved.Status != WaitStatusResolved || resolved.WakeState != WakeStatePending {
		t.Fatalf("dependency wait status=%s wake=%s", resolved.Status, resolved.WakeState)
	}
	updated, _ := repo.GetThread(thread.ID, "space-1")
	if updated.CurrentState != "任务执行已完成" {
		t.Fatalf("current state=%q", updated.CurrentState)
	}
}

func TestRuntimeBridgeApprovalLifecycle(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "审批持续事项", Status: ThreadStatusActive}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	if err := repo.Bind(thread.ID, "request", "req-1", "request", "test", 1); err != nil {
		t.Fatal(err)
	}
	coordinator := NewWaitCoordinator(repo, &recordingWakeDispatcher{}, DefaultWaitCoordinatorConfig())
	bridge := NewRuntimeBridge(repo, coordinator)
	bridge.OnApprovalRequested(context.Background(), ApprovalLifecycleSignal{
		ApprovalID: "approval-1", SpaceID: "space-1", RequestID: "req-1", ToolCallID: "tool-1",
	})
	waits, _ := repo.ListOpenWaits(thread.ID, 10)
	if len(waits) != 1 || waits[0].WaitType != WaitTypeApproval {
		t.Fatalf("waits=%#v, want one approval wait", waits)
	}
	bridge.OnApprovalResolved(context.Background(), ApprovalLifecycleSignal{
		Resolved: true, Approved: true, ApprovalID: "approval-1", SpaceID: "space-1",
		RequestID: "req-1", ToolCallID: "tool-1", Timestamp: time.Now().UTC(),
	})
	resolved, _ := repo.GetWait(waits[0].ID)
	if resolved.Status != WaitStatusResolved || resolved.WakeState != WakeStateSkipped {
		t.Fatalf("approval wait status=%s wake=%s", resolved.Status, resolved.WakeState)
	}
	updated, _ := repo.GetThread(thread.ID, "space-1")
	if updated.Status != ThreadStatusActive {
		t.Fatalf("thread status=%s, want active", updated.Status)
	}
}

func TestRuntimeBridgeDeviceReadyResolvesDeviceWait(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "设备持续事项", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeDevice, Status: WaitStatusWaiting, ConditionJSON: `{"deviceId":"device-1"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	coordinator := NewWaitCoordinator(repo, &recordingWakeDispatcher{}, DefaultWaitCoordinatorConfig())
	bridge := NewRuntimeBridge(repo, coordinator)
	bridge.OnDeviceReady(context.Background(), "space-1", "device-1")
	resolved, _ := repo.GetWait(wait.ID)
	if resolved.Status != WaitStatusResolved || resolved.WakeState != WakeStatePending {
		t.Fatalf("device wait status=%s wake=%s", resolved.Status, resolved.WakeState)
	}
}
