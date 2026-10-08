package task_runtime

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestRemotePauseRequiresMatchingOwnerCheckpointAndCurrentAttempt(t *testing.T) {
	for _, scenario := range []string{"valid", "attempt", "lease", "version", "missing", "corrupt", "status", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			service, _, authority, run, definition := taskAuthorityFixture(t)
			definition.Checkpoint, definition.DefinitionHash = true, "core-definition"
			run.TaskDefinitionID = definition.TaskID
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			run.Status, run.Generation, run.Revision, run.ExecutionAttemptID, run.LeaseID = RunStatusPausing, 4, 8, "attempt", "lease"
			run.ExecutionPlacement = TaskExecutionPlacementDevice
			run.Input = json.RawMessage(`{"private":"device-input"}`)
			run.InputHash = hashBytes(run.Input)
			data := &taskInputData{}
			ctx := coordination.WithScope(t.Context(), authority)
			if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
				t.Fatal(err)
			}
			run.Input = nil
			store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
			service.store = store
			service.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
			service.config.OwnedExecutionGuard = func(ctx context.Context, authority coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
				guarded := coordination.WithScope(ctx, authority)
				if scenario == "expired" {
					guarded = coordination.WithAdditionalGuard(guarded, func(context.Context) error { return coordination.ErrScopeExpired })
				}
				return guarded, func() {}, nil
			}
			payload := json.RawMessage(`{"cursor":2,"private":"only-at-device"}`)
			if err := service.handleCheckpoint(ctx, run, definition, payload, hashBytes(payload), 2); err != nil {
				t.Fatal(err)
			}
			attempt, lease, version := "attempt", "lease", int64(2)
			switch scenario {
			case "attempt":
				attempt = "old-attempt"
			case "lease":
				lease = "old-lease"
			case "version":
				version++
			case "missing":
				store.run.CheckpointID = nil
			case "corrupt":
				store.checkpoint.PayloadHash = hashBytes([]byte("foreign"))
			case "status":
				store.run.Status = RunStatusRunning
			}
			err := service.HandleRemotePaused(t.Context(), run.TaskRunID, attempt, lease, version)
			if scenario == "valid" {
				if err != nil || store.run.Status != RunStatusPaused || store.run.PausedAt == nil || store.run.FinishedAt != nil || store.result != nil || len(store.checkpoint.Payload) != 0 {
					t.Fatalf("owner pause was not confirmed without data mirror: %v %+v", err, store.run)
				}
				revision := store.run.Revision
				if err := service.HandleRemotePaused(t.Context(), run.TaskRunID, attempt, lease, version); err != nil || store.run.Revision != revision {
					t.Fatalf("pause acknowledgement retry changed state: %v", err)
				}
				store.checkpoint.PayloadHash = hashBytes([]byte("changed"))
				if err := service.HandleRemotePaused(t.Context(), run.TaskRunID, attempt, lease, version); err == nil {
					t.Fatal("pause retry ignored changed owner checkpoint")
				}
			} else if err == nil || store.run.Status == RunStatusPaused {
				t.Fatalf("invalid pause was accepted: %s %v", scenario, err)
			}
		})
	}
}

func TestRemoteResumeRejectsUnconfirmedStopAndClearsOldLease(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	definition.Checkpoint, definition.DefinitionHash = true, "hash"
	definition.PermissionRequirementStrings = []string{"test.source.read"}
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Status, run.Generation, run.Revision, run.ExecutionAttemptID, run.LeaseID = RunStatusPausing, 3, 7, "attempt", "lease"
	run.ExecutionPlacement = TaskExecutionPlacementDevice
	run.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID), RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 7}
	run.Input, run.InputHash = nil, hashBytes(json.RawMessage(`{}`))
	data := &taskInputData{}
	input := CloneTaskRun(run)
	input.Input = json.RawMessage(`{}`)
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, input); err != nil {
		t.Fatal(err)
	}
	store := &pauseStore{run: run, def: definition}
	service.store, service.queue = store, NewTaskQueue(store, "test", time.Minute)
	service.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	service.config.OwnedInputs = AcknowledgedTaskInputPort{Data: data}
	service.config.OwnedTargetDefinitions = ownedDefinitionPortFunc(func(context.Context, *TaskRun, *TaskDefinition) (TargetTaskDefinitionPin, error) {
		return TargetTaskDefinitionPin{}, nil
	})
	var resourceApproved bool
	service.config.OwnedTargetPermissions = ownedPermissionPortFunc(func(_ context.Context, next *TaskRun, _ *TaskDefinition, _ TargetTaskDefinitionPin, input json.RawMessage) error {
		if next.Generation != 4 || string(input) != `{}` || len(next.Input) != 0 {
			t.Fatal("恢复资源审批未绑定新执行代次或复制了输入正文")
		}
		if !resourceApproved {
			return NewTaskError(ErrTaskPermissionDenied, "恢复执行需要新审批")
		}
		return nil
	})
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	payload := json.RawMessage(`{"cursor":2}`)
	if err := service.handleCheckpoint(ctx, run, definition, payload, hashBytes(payload), 2); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleRemotePaused(t.Context(), run.TaskRunID, "attempt", "lease", 2); err != nil {
		t.Fatal(err)
	}
	manager := NewPendingTaskManager()
	remote := NewMeshRemoteTaskExecutor(nil, nil, manager)
	service.SetRemoteExecutor(remote)
	confirmedAt := store.run.PausedAt
	store.run.PausedAt = nil
	if err := service.ResumeTask(t.Context(), ResumeTaskRequest{TaskRunID: run.TaskRunID, Generation: 3}); !IsTaskErrorCode(err, ErrTaskPauseInProgress) {
		t.Fatalf("unconfirmed stop resumed: %v", err)
	}
	store.run.PausedAt = confirmedAt
	if err := service.recoverRun(t.Context(), store.run); err != nil || store.run.Status != RunStatusPaused || store.queued != 0 {
		t.Fatalf("restart replayed or discarded a confirmed pause: %v", err)
	}
	atomic.StoreInt32(&service.dispatching, 1)
	if err := service.ResumeTask(t.Context(), ResumeTaskRequest{TaskRunID: run.TaskRunID, Generation: 3}); !IsTaskErrorCode(err, ErrTaskPermissionDenied) || store.run.Status != RunStatusPaused || store.run.Generation != 3 || store.queued != 0 {
		t.Fatalf("资源未批准的恢复改变了暂停状态或排队: %v %+v", err, store.run)
	}
	resourceApproved = true
	if err := service.ResumeTask(t.Context(), ResumeTaskRequest{TaskRunID: run.TaskRunID, Generation: 3}); err != nil {
		t.Fatal(err)
	}
	if store.run.Status != RunStatusQueued || store.run.Generation != 4 || store.run.ExecutionAttemptID != "" || store.run.LeaseID != "" || store.run.LeaseExpiresAt != nil || store.queued != 1 || store.run.CheckpointID == nil {
		t.Fatalf("resume retained old execution or lost checkpoint: %+v", store.run)
	}
}

func TestConfirmedPauseReceiptDoesNotRepeatRoleRPCInsideShortWait(t *testing.T) {
	_, _, authority, run, _ := taskAuthorityFixture(t)
	now := time.Now().UTC()
	run.Status, run.PausedAt, run.Generation = RunStatusPaused, &now, 1
	run.ExecutionAttemptID, run.LeaseID = "attempt", "lease"
	run.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID), RuntimeSessionID: "session", ConnectionGeneration: 1}
	checks := 0
	ctx := coordination.WithAdditionalGuard(coordination.WithScope(t.Context(), authority), func(context.Context) error { checks++; return nil })
	executor := NewMeshRemoteTaskExecutor(nil, nil, NewPendingTaskManager())
	if err := coordination.ValidateCurrent(ctx); err != nil {
		t.Fatal(err)
	}
	if !executor.ConfirmPauseStopped(ctx, run) || checks != 1 {
		t.Fatal("停止回执复核重复执行了远端角色查询")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil || checks != 2 {
		t.Fatal("调用者在停止确认后不能重新复核当前权限")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if executor.ConfirmPauseStopped(cancelled, run) {
		t.Fatal("失效请求确认了停止凭证")
	}
	foreign := authority
	foreign.TargetDeviceID = "other"
	if executor.ConfirmPauseStopped(coordination.WithScope(ctx, foreign), run) {
		t.Fatal("其他设备确认了此任务的停止凭证")
	}
}
