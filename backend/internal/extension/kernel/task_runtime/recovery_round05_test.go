package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type startupRecoveryStore struct {
	pauseStore
	readError error
	wrote     int
}

func (s *startupRecoveryStore) PutTaskRun(_ context.Context, run *TaskRun) error {
	s.run = CloneTaskRun(run)
	s.wrote++
	return nil
}

func (s *startupRecoveryStore) GetLatestCheckpoint(ctx context.Context, id string) (*TaskCheckpoint, error) {
	if s.readError != nil {
		return nil, s.readError
	}
	return s.pauseStore.GetLatestCheckpoint(ctx, id)
}

func TestStartupRecoveryReadsAndValidatesCheckpointBeforeStateChange(t *testing.T) {
	for _, situation := range []string{"valid", "read-failure", "damaged-body", "different-run", "different-pin", "missing"} {
		t.Run(situation, func(t *testing.T) {
			payload := json.RawMessage(`{"cursor":1}`)
			pin := "cp-1"
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", InputHash: "input", Status: RunStatusRunning, CheckpointID: &pin, Generation: 2, Revision: 5}
			def := &TaskDefinition{TaskID: "task", DefinitionHash: "definition", Recoverability: CheckpointRecoverable}
			cp := &TaskCheckpoint{CheckpointID: pin, TaskRunID: run.TaskRunID, Version: 1, InputHash: run.InputHash, DefinitionHash: def.DefinitionHash, Payload: payload, PayloadHash: hashBytes(payload)}
			store := &startupRecoveryStore{pauseStore: pauseStore{run: run, def: def, cp: cp}}
			switch situation {
			case "read-failure":
				store.readError = errors.New("db unavailable")
			case "damaged-body":
				cp.Payload[0] = '['
			case "different-run":
				cp.TaskRunID = "foreign"
			case "different-pin":
				cp.CheckpointID = "cp-2"
			case "missing":
				store.cp = nil
			}
			svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			err := svc.recoverRun(t.Context(), run)
			if situation == "read-failure" {
				if err == nil || !strings.Contains(err.Error(), "db unavailable") || store.wrote != 0 || store.run.Status != RunStatusRunning {
					t.Fatalf("storage failure changed durable state: err=%v run=%+v writes=%d", err, store.run, store.wrote)
				}
				return
			}
			if err != nil || store.wrote != 1 || store.queued != 0 {
				t.Fatalf("unexpected startup result: err=%v run=%+v writes=%d queued=%d", err, store.run, store.wrote, store.queued)
			}
			expected := RunStatusManualIntervention
			if situation == "valid" {
				expected = RunStatusRecoveryRequired
			}
			if store.run.Status != expected {
				t.Fatalf("unexpected startup recovery status: %v wanted %v", store.run.Status, expected)
			}
		})
	}
}

func TestStartupRecoveryPreservesConfirmedLocalPause(t *testing.T) {
	payload := json.RawMessage(`{"phase":"paused"}`)
	pin, now := "confirmed", time.Now().UTC()
	run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", InputHash: "input", Status: RunStatusPaused, CheckpointID: &pin, PausedAt: &now, ExecutionPlacement: TaskExecutionPlacementLocal}
	def := &TaskDefinition{TaskID: "task", DefinitionHash: "definition", Recoverability: CheckpointRecoverable}
	cp := &TaskCheckpoint{CheckpointID: pin, TaskRunID: "run", Version: 2, InputHash: "input", DefinitionHash: "definition", Payload: payload, PayloadHash: hashBytes(payload)}
	store := &startupRecoveryStore{pauseStore: pauseStore{run: run, def: def, cp: cp}}
	if err := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig()).recoverRun(t.Context(), run); err != nil || store.wrote != 0 || run.Status != RunStatusPaused {
		t.Fatalf("confirmed pause was reopened: %v %+v", err, run)
	}
}

func TestStartupNeverResendsInterruptedRemoteTask(t *testing.T) {
	for _, placement := range []TaskExecutionPlacement{TaskExecutionPlacementDevice, TaskExecutionPlacementCloud} {
		t.Run(string(placement), func(t *testing.T) {
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusRunning, ExecutionPlacement: placement, ExecutionAttemptID: "attempt", LeaseID: "old", Attempt: 1, MaxAttempts: 3}
			def := &TaskDefinition{TaskID: "task", Recoverability: RestartableFromBeginning, Idempotency: Idempotent}
			store := &startupRecoveryStore{pauseStore: pauseStore{run: run, def: def}}
			if err := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig()).recoverRun(t.Context(), run); err != nil || store.queued != 0 || store.run.Status != RunStatusRecoveryRequired {
				t.Fatalf("remote attempt was restarted without stop confirmation: %v %+v queued=%d", err, store.run, store.queued)
			}
		})
	}
}

func TestStartupLocalIdempotentRestartRetainsExistingQueue(t *testing.T) {
	run := &TaskRun{TaskRunID: "local", TaskDefinitionID: "task", Status: RunStatusRunning, ExecutionPlacement: TaskExecutionPlacementLocal}
	def := &TaskDefinition{TaskID: "task", Recoverability: RestartableFromBeginning, Idempotency: Idempotent}
	store := &startupRecoveryStore{pauseStore: pauseStore{run: run, def: def}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	if err := svc.recoverRun(t.Context(), run); err != nil || store.run.Status != RunStatusQueued || store.queued != 1 {
		t.Fatalf("safe local restart failed: %v %+v queued=%d", err, store.run, store.queued)
	}
}

func TestRemoteRecoveryReceiptAloneDoesNotReplaceOwnerAuthority(t *testing.T) {
	for _, scenario := range []string{"missing", "wrong-attempt", "wrong-lease", "wrong-session", "wrong-generation", "wrong-device", "wrong-runtime", "active", "confirmed"} {
		t.Run(scenario, func(t *testing.T) {
			run := &TaskRun{
				TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusRecoveryRequired,
				Generation: 3, Revision: 5, ExecutionPlacement: TaskExecutionPlacementDevice,
				ExecutionAttemptID: "old-attempt", ExecutionTarget: TaskExecutionTarget{SpaceID: "space", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 7},
			}
			def := &TaskDefinition{TaskID: "task", Recoverability: RestartableFromBeginning, Idempotency: Idempotent}
			store := &pauseStore{run: run, def: def}
			manager := NewPendingTaskManager()
			remote := NewMeshRemoteTaskExecutor(nil, nil, manager)
			svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			svc.SetRemoteExecutor(remote)
			svc.queue = NewTaskQueue(store, "test-recovery", time.Minute)
			atomic.StoreInt32(&svc.dispatching, 1)
			if scenario != "missing" {
				request := TaskExecutionRequest{Run: run, AttemptID: run.ExecutionAttemptID, Target: run.ExecutionTarget}
				pending, err := manager.Register(request, "session", 7, time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				run.LeaseID = pending.LeaseID
				if scenario != "active" {
					if !manager.CompleteBound("run", "old-attempt", pending.LeaseID, "session", 7, false, "stopped") {
						t.Fatal("could not confirm old attempt")
					}
				}
			}

			past := time.Now().UTC().Add(-time.Minute)
			run.LeaseExpiresAt = &past
			switch scenario {
			case "wrong-attempt":
				run.ExecutionAttemptID = "different"
			case "wrong-lease":
				run.LeaseID = "different"
			case "wrong-session":
				run.ExecutionTarget.RuntimeSessionID = "different"
			case "wrong-generation":
				run.ExecutionTarget.ConnectionGeneration = 8
			case "wrong-device":
				run.ExecutionTarget.DeviceID = "foreign-device"
			case "wrong-runtime":
				run.ExecutionTarget.RuntimeID = "foreign-runtime"
			}
			recovered, err := svc.Recover(t.Context(), "run")
			if err == nil || recovered != nil || store.run.Status != RunStatusRecoveryRequired || store.queued != 0 {
				t.Fatalf("stop receipt alone replaced missing Owner authority: %s %v %+v", scenario, err, store.run)
			}
			manager.Cancel("run", "cleanup")
		})
	}
}

func TestCancelAcknowledgementCannotBorrowAnotherAttempt(t *testing.T) {
	manager := NewPendingTaskManager()
	run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionAttemptID: "attempt-a", ExecutionTarget: TaskExecutionTarget{RuntimeSessionID: "session-a", ConnectionGeneration: 2}}
	request := TaskExecutionRequest{Run: run, AttemptID: run.ExecutionAttemptID, Target: run.ExecutionTarget}
	first, err := manager.Register(request, "session-a", 2, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	run.LeaseID = first.LeaseID
	if !manager.CompleteBound("run", "attempt-a", first.LeaseID, "session-a", 2, true, "") {
		t.Fatal("could not acknowledge the first worker")
	}
	if !manager.WaitForCancelAckBound(t.Context(), run) {
		t.Fatal("first worker confirmation missing")
	}
	oldRun := CloneTaskRun(run)
	run.ExecutionAttemptID = "attempt-b"
	run.ExecutionTarget.RuntimeSessionID = "session-b"
	run.ExecutionTarget.ConnectionGeneration = 3
	second, err := manager.Register(TaskExecutionRequest{Run: run, AttemptID: run.ExecutionAttemptID, Target: run.ExecutionTarget}, "session-b", 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	run.LeaseID = second.LeaseID
	waitCtx, stop := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer stop()
	if manager.WaitForCancelAckBound(waitCtx, oldRun) || manager.WaitForCancelAckBound(waitCtx, run) {
		t.Fatal("old confirmation was applied to a new or active worker")
	}
	if !manager.CompleteBound("run", "attempt-b", second.LeaseID, "session-b", 3, true, "") {
		t.Fatal("could not acknowledge second worker")
	}
	if manager.WaitForCancelAckBound(t.Context(), oldRun) || !manager.WaitForCancelAckBound(t.Context(), run) {
		t.Fatal("completion was not bound to the correct attempt")
	}
}
