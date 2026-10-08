package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ownedDeduplicationStore struct {
	taskInputQueueStore
	creates    int
	definition *TaskDefinition
}

func (s *ownedDeduplicationStore) GetTaskDefinition(ctx context.Context, id string) (*TaskDefinition, error) {
	if s.definition != nil {
		return s.definition, nil
	}
	return s.taskInputQueueStore.GetTaskDefinition(ctx, id)
}

func (s *ownedDeduplicationStore) GetTaskRun(context.Context, string) (*TaskRun, error) {
	if s.run == nil {
		return nil, NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	return CloneTaskRun(s.run), nil
}

func (s *ownedDeduplicationStore) CreateTaskRun(_ context.Context, run *TaskRun) (bool, error) {
	if s.run != nil {
		return false, nil
	}
	s.run, s.creates = CloneTaskRun(run), s.creates+1
	return true, nil
}

func TestOwnedEnqueueDeduplicatesRequestWithoutRepeatingOrReplacingExecution(t *testing.T) {
	service, _, authority, saved, definition := taskAuthorityFixture(t)
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	data := &taskInputData{}
	service.config.OwnedInputs = AcknowledgedTaskInputPort{Data: data}
	store := &ownedDeduplicationStore{}
	service.store, service.queue = store, NewTaskQueue(store, "deduplication", service.config.LeaseDuration)
	atomic.StoreInt32(&service.dispatching, 1)
	ctx := coordination.WithScope(t.Context(), authority)
	request := EnqueueTaskRequest{DeduplicateOwnedRequest: true, ScopeSnapshotID: saved.ScopeSnapshotID, InvocationID: saved.InvocationID, Input: json.RawMessage(`{"private":"original"}`)}
	first, err := service.Enqueue(ctx, request, definition)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []TaskRunStatus{RunStatusQueued, RunStatusRunning, RunStatusRecoveryRequired, RunStatusSucceeded} {
		store.run.Status = status
		store.run.Revision, store.run.Generation, store.run.ExecutionAttemptID = 9, 3, "original-attempt"
		result, err := service.Enqueue(ctx, request, definition)
		if err != nil || result.TaskRunID != first.TaskRunID || result.Status != status || result.Queued != (status == RunStatusQueued) || store.queued != 1 || store.creates != 1 || store.run.Revision != 9 || store.run.Generation != 3 || store.run.ExecutionAttemptID != "original-attempt" {
			t.Fatalf("请求重试重复执行或覆盖已有任务: %+v %+v %v", result, store.run, err)
		}
	}
	request.Input = json.RawMessage(`{"private":"changed"}`)
	if _, err := service.Enqueue(ctx, request, definition); !errors.Is(err, coordination.ErrRequestConflict) || store.queued != 1 {
		t.Fatalf("同一请求接受了不同输入: %v", err)
	}
	request.Input = json.RawMessage(`{"private":"original"}`)
	denied := coordination.WithAdditionalGuard(ctx, func(context.Context) error { return coordination.ErrScopeExpired })
	if _, err := service.Enqueue(denied, request, definition); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("请求重试绕过当前权限: %v", err)
	}
	data.resource.Deleted = true
	if _, err := service.Enqueue(ctx, request, definition); err == nil {
		t.Fatal("请求重试接受了已经删除的所有者输入")
	}
}

func TestOwnedEnqueueDeduplicationCannotBeEnabledByClientOrCrossDevice(t *testing.T) {
	var request EnqueueTaskRequest
	if err := json.Unmarshal([]byte(`{"DeduplicateOwnedRequest":true,"deduplicateOwnedRequest":true}`), &request); err != nil || request.DeduplicateOwnedRequest {
		t.Fatal("客户端能够直接指定内部请求去重策略")
	}
	base := coordination.ExecutionScope{CoreID: "core", SpaceID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", RequestID: "request"}
	id, err := OwnedRequestTaskRunID(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"core", "caller", "request"} {
		changed := base
		switch scenario {
		case "core":
			changed.CoreID, changed.SpaceID, changed.AuthorizationRealm = "other", "other", "other"
		case "caller":
			changed.InitiatorDeviceID = "other"
		case "request":
			changed.RequestID = "other"
		}
		other, err := OwnedRequestTaskRunID(changed)
		if err != nil || other == id {
			t.Fatalf("请求编号串用其他 Core、设备或请求: %s %v", scenario, err)
		}
	}
	if _, err := OwnedRequestTaskRunID(coordination.ExecutionScope{}); err == nil {
		t.Fatal("没有已认证归属也能计算任务请求身份")
	}
}

func TestExistingOwnedDeviceTaskRequestReadsConfirmedStateWithoutSourcePermissionOrDispatch(t *testing.T) {
	service, snapshots, authority, _, definition := taskAuthorityFixture(t)
	definition.ExecutionPlacement = TaskExecutionPlacementDevice
	definition.PermissionRequirementStrings = []string{"source.per-use"}
	ctx := coordination.WithScope(t.Context(), authority)
	id, err := OwnedRequestTaskRunID(authority)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(authority)
	if err := snapshots.SaveSnapshot(ctx, scope.ScopeSnapshot{SnapshotID: id, InvocationID: id, SpaceID: authority.CoreID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, CharacterID: authority.RoleID, OwnedExecutionScope: encoded}); err != nil {
		t.Fatal(err)
	}
	input := json.RawMessage(`{"private":"confirmed"}`)
	fingerprint, _ := taskDefinitionFingerprint(definition)
	run := &TaskRun{TaskRunID: id, InvocationID: id, ScopeSnapshotID: id, TaskDefinitionID: definition.TaskID, DefinitionFingerprint: fingerprint, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InputHash: hashBytes(input), Input: input, ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionTarget: TaskExecutionTarget{SpaceID: runtimeidentity.SpaceID(authority.CoreID), DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID), RuntimeID: "old-runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 1}, Revision: 9, Generation: 3, ExecutionAttemptID: "confirmed-attempt"}
	data := &taskInputData{}
	service.config.OwnedInputs = AcknowledgedTaskInputPort{Data: data}
	if err := service.config.OwnedInputs.SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	store := &ownedDeduplicationStore{taskInputQueueStore: taskInputQueueStore{run: run}, definition: definition}
	service.store = store
	for _, status := range []TaskRunStatus{RunStatusQueued, RunStatusRunning, RunStatusPaused, RunStatusRecoveryRequired, RunStatusSucceeded} {
		store.run.Status = status
		result, err := service.ExistingOwnedDeviceTaskRequest(ctx, definition.TaskID, input, nil)
		if err != nil || result == nil || result.Status != status || result.TaskRunID != id || result.Queued != (status == RunStatusQueued) || store.queued != 0 || store.creates != 0 || store.run.Revision != 9 || store.run.Generation != 3 || store.run.ExecutionAttemptID != "confirmed-attempt" {
			t.Fatalf("状态确认依赖已消费审批、重复调度或修改任务: %+v %v", result, err)
		}
	}
	if _, err := service.ExistingOwnedDeviceTaskRequest(ctx, definition.TaskID, json.RawMessage(`{"private":"changed"}`), nil); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("确认接口接受了替换输入: %v", err)
	}
	if _, err := service.ExistingOwnedDeviceTaskRequest(ctx, "other-task", input, nil); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("确认接口接受了替换任务: %v", err)
	}
	if _, err := service.ExistingOwnedDeviceTaskRequest(ctx, definition.TaskID, input, &DeviceTaskDefinitionReference{CatalogID: definition.TaskID}); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("确认接口接受了替换目录引用: %v", err)
	}
	changed := authority
	changed.RoleRevision++
	if _, err := service.ExistingOwnedDeviceTaskRequest(coordination.WithScope(t.Context(), changed), definition.TaskID, input, nil); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("确认接口接受了改变的角色版本: %v", err)
	}
	denied := coordination.WithAdditionalGuard(ctx, func(context.Context) error { return coordination.ErrScopeExpired })
	if _, err := service.ExistingOwnedDeviceTaskRequest(denied, definition.TaskID, input, nil); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("确认接口绕过撤权: %v", err)
	}
	data.resource.Deleted = true
	if _, err := service.ExistingOwnedDeviceTaskRequest(ctx, definition.TaskID, input, nil); err == nil {
		t.Fatal("确认接口接受了已删除的所有者输入")
	}
}
