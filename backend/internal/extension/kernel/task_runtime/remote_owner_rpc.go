package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type RemoteTaskOwnerRequest = protocol.TaskOwnerRPCRequest

func taskOwnerExecutionActive(status TaskRunStatus) bool {
	return status == RunStatusRunning || status == RunStatusCheckpointing || status == RunStatusPausing
}

func (s *TaskRuntimeService) lockTaskOwner(taskID string) func() {
	hash := sha256.Sum256([]byte(taskID))
	lock := &s.ownerLocks[int(hash[0])%len(s.ownerLocks)]
	lock.Lock()
	var once sync.Once
	return func() { once.Do(lock.Unlock) }
}

func (s *TaskRuntimeService) CallRemoteOwner(ctx context.Context, taskID string, request RemoteTaskOwnerRequest, checkBinding func() error) (json.RawMessage, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.PrincipalType != auth.PrincipalTrustedDevice || actor.DeviceID == "" || actor.RuntimeID == "" || taskID == "" || len(taskID) > 256 || request.AuthorityCallID == "" || len(request.AuthorityCallID) > 512 || request.RequestID == "" || len(request.RequestID) > 256 || !json.Valid(request.Params) || len(request.Params) > 1500<<10 || checkBinding == nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者接口缺少已配对执行设备或完整请求")
	}
	if err := checkBinding(); err != nil {
		return nil, err
	}
	unlock := s.lockTaskOwner(taskID)
	defer unlock()
	run, err := s.store.GetTaskRun(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if run == nil || run.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice || run.Generation != request.TaskGeneration || run.ExecutionAttemptID.String() != request.AttemptID || request.LeaseID == "" || run.ExecutionTarget.SpaceID != actor.SpaceID || run.ExecutionTarget.DeviceID != actor.DeviceID || run.ExecutionTarget.RuntimeID != actor.RuntimeID || run.ExecutionTarget.RuntimeSessionID.String() != request.SessionID || run.ExecutionTarget.ConnectionGeneration != request.ConnectionGeneration || !taskOwnerExecutionActive(run.Status) {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "任务所有者接口的执行设备、租约或连接已失效")
	}
	guarded, finish, owned, err := s.callbackAuthority(ctx, run, request.AttemptID, request.TaskGeneration)
	if err != nil {
		return nil, err
	}
	defer finish()
	authority, inherited := coordination.FromContext(guarded)
	owner := authority.TargetDeviceID
	if authority.Coordinated {
		owner = authority.CoreID
	}
	if !owned || !inherited || authority != request.Scope || owner == "" || authority.ResourceOwnerID != owner || authority.RoleOwnerID != owner || authority.TargetDeviceID != actor.DeviceID.String() {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者接口与原 Core 授权范围不一致")
	}
	guarded = coordination.WithAdditionalGuard(guarded, func(context.Context) error { return checkBinding() })
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return nil, err
	}
	var result json.RawMessage
	switch {
	case request.Method == "task.host.executeTool" || request.Method == "task.host.emitEvent":
		if s.config.OwnedHost == nil {
			return nil, NewTaskError(ErrTaskDependencyUnavailable, "任务Native所有者授信端口未就绪")
		}
		definition, readErr := s.store.GetTaskDefinition(guarded, run.TaskDefinitionID)
		if readErr != nil {
			return nil, readErr
		}
		fingerprint, readErr := taskDefinitionFingerprint(definition)
		if readErr != nil || fingerprint != run.DefinitionFingerprint {
			return nil, NewTaskError(ErrTaskDefinitionInvalid, "Native调用与原任务定义指纹不一致")
		}
		result, err = s.config.OwnedHost.Call(guarded, run, definition, request.RequestID, request.Method, request.Params)
	case request.Method == "task.progress.save":
		var input struct {
			TaskRunID  string   `json:"task_run_id"`
			Sequence   int64    `json:"sequence"`
			Current    *float64 `json:"current"`
			Total      *float64 `json:"total"`
			Percentage *float64 `json:"percentage"`
			Stage      string   `json:"stage"`
			Message    string   `json:"message"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.TaskRunID != taskID {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度归属无效")
		}
		if err := s.persistTaskProgress(guarded, taskID, input.Sequence, input.Current, input.Total, input.Percentage, input.Stage, input.Message, true, run); err != nil {
			return nil, err
		}
		result = json.RawMessage(`{"confirmed":true}`)
	case request.Method == "task.checkpoint.save":
		var input struct {
			TaskRunID string          `json:"task_run_id"`
			Version   int64           `json:"version"`
			Payload   json.RawMessage `json:"payload"`
		}
		var checkpoint struct {
			Cursor int64 `json:"cursor"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.TaskRunID != taskID || input.Version < 1 || input.Version > 9007199254740991 || len(input.Payload) > 1<<20 || json.Unmarshal(input.Payload, &checkpoint) != nil || checkpoint.Cursor != input.Version {
			return nil, NewTaskError(ErrTaskCheckpointHashMismatch, "任务检查点归属或版本无效")
		}
		definition, readErr := s.store.GetTaskDefinition(guarded, run.TaskDefinitionID)
		if readErr != nil {
			return nil, readErr
		}
		if err := s.handleCheckpoint(guarded, run, definition, input.Payload, hashBytes(input.Payload), input.Version); err != nil {
			return nil, err
		}
		result, err = json.Marshal(map[string]int64{"version": input.Version})
	case strings.HasPrefix(request.Method, "task.artifact."):
		if s.config.OwnedArtifacts == nil {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务产物所有者端口不可用")
		}
		result, err = s.config.OwnedArtifacts.Call(guarded, run, request.RequestID, request.Method, request.Params)
	case request.Method == "task.storage.get" || request.Method == "task.storage.set" || request.Method == "task.storage.delete":
		if s.config.OwnedStorage == nil {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务存储所有者端口不可用")
		}
		result, err = s.config.OwnedStorage.Call(guarded, run, request.RequestID, request.Method, request.Params)
	default:
		return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者接口未授予此操作")
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(result) || len(result) > 512<<10 {
		return nil, coordination.ErrPendingLimit
	}
	latest, err := s.store.GetTaskRun(guarded, taskID)
	if err != nil {
		return nil, err
	}
	if latest == nil || latest.Generation != run.Generation || latest.ExecutionAttemptID != run.ExecutionAttemptID || latest.Status != run.Status || latest.ExecutionTarget != run.ExecutionTarget {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "任务所有者确认期间执行状态已变化")
	}
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return nil, err
	}
	return result, nil
}
