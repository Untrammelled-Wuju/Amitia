package task_runtime

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/server"
)

func TestRound06RemoteCancelRequiresOriginalExecutionBinding(t *testing.T) {
	remote := NewMeshRemoteTaskExecutor(server.NewConnectionHub(), nil)
	for _, scenario := range []struct {
		name   string
		change func(*TaskRun)
	}{
		{"missing-run", func(r *TaskRun) {}},
		{"missing-attempt", func(r *TaskRun) { r.ExecutionAttemptID = "" }},
		{"missing-lease", func(r *TaskRun) { r.LeaseID = "" }},
		{"missing-session", func(r *TaskRun) { r.ExecutionTarget.RuntimeSessionID = "" }},
		{"missing-generation", func(r *TaskRun) { r.ExecutionTarget.ConnectionGeneration = 0 }},
		{"missing-runtime", func(r *TaskRun) { r.ExecutionTarget.RuntimeID = "" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.name == "missing-run" {
				if err := remote.Cancel(t.Context(), nil); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
					t.Fatalf("missing run was not blocked: %v", err)
				}
				return
			}
			run := &TaskRun{
				TaskRunID: "run", ExecutionAttemptID: "old-attempt", LeaseID: "old-lease",
				ExecutionPlacement: TaskExecutionPlacementDevice,
				ExecutionTarget:    TaskExecutionTarget{SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 7},
			}
			scenario.change(run)
			err := remote.Cancel(t.Context(), run)
			if err == nil {
				t.Fatal("cancellation without original worker binding was dispatched")
			}
			if scenario.name == "missing-runtime" && !IsTaskErrorCode(err, ErrTaskDeviceBindingInvalid) {
				t.Fatalf("expected device binding rejection: %v", err)
			}
		})
	}
}

func TestRound06CancelAndWaitRequiresConfirmationBackend(t *testing.T) {
	remote := &MeshRemoteTaskExecutor{hub: server.NewConnectionHub()}
	run := &TaskRun{
		TaskRunID: "run", ExecutionAttemptID: "attempt", LeaseID: "lease",
		ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget:    TaskExecutionTarget{SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 7},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := remote.CancelAndWait(ctx, run, time.Millisecond); !IsTaskErrorCode(err, ErrRemoteTaskExecutorUnavailable) {
		t.Fatalf("missing worker confirmation backend was reported as stopped: %v", err)
	}
	var absent *MeshRemoteTaskExecutor
	if err := absent.CancelAndWait(ctx, run, time.Millisecond); !IsTaskErrorCode(err, ErrRemoteTaskExecutorUnavailable) {
		t.Fatalf("missing remote executor was reported as stopped: %v", err)
	}
}

func TestRound06PendingTaskBindingNeverAllowsWildcards(t *testing.T) {
	manager := NewPendingTaskManager()
	run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionAttemptID: "attempt-a", ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 2}}
	pending, err := manager.Register(TaskExecutionRequest{Run: run, AttemptID: run.ExecutionAttemptID, Target: run.ExecutionTarget}, "session", 2, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Cancel("run", "test cleanup")
	for _, binding := range []struct {
		name                string
		attempt, lease, ses string
		generation          int64
	}{
		{"blank-attempt", "", pending.LeaseID, "session", 2},
		{"blank-lease", "attempt-a", "", "session", 2},
		{"blank-session", "attempt-a", pending.LeaseID, "", 2},
		{"blank-generation", "attempt-a", pending.LeaseID, "session", 0},
		{"old-attempt", "attempt-old", pending.LeaseID, "session", 2},
		{"old-lease", "attempt-a", "old-lease", "session", 2},
		{"old-session", "attempt-a", pending.LeaseID, "old-session", 2},
		{"old-generation", "attempt-a", pending.LeaseID, "session", 1},
	} {
		t.Run(binding.name, func(t *testing.T) {
			if manager.ValidateBound("run", binding.attempt, binding.lease, binding.ses, binding.generation) ||
				manager.ClaimBound("run", binding.attempt, binding.lease, binding.ses, binding.generation, "worker", time.Minute) ||
				manager.HeartbeatBound("run", binding.attempt, binding.lease, binding.ses, binding.generation, 1, time.Minute) {
				t.Fatal("unbound callback was admitted")
			}
		})
	}
	if !manager.ValidateBound("run", "attempt-a", pending.LeaseID, "session", 2) ||
		!manager.ClaimBound("run", "attempt-a", pending.LeaseID, "session", 2, "worker", time.Minute) ||
		!manager.HeartbeatBound("run", "attempt-a", pending.LeaseID, "session", 2, 1, time.Minute) {
		t.Fatal("valid worker binding was refused")
	}
}
