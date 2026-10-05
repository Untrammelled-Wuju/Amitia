package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type SourceTaskOwnerCall func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)

func (s *TaskRuntimeService) ExecuteOwnedSourceDispatch(ctx context.Context, dispatch protocol.TaskDispatchPayload, call SourceTaskOwnerCall) (protocol.OwnedTaskExecutionOutcome, error) {
	empty := protocol.OwnedTaskExecutionOutcome{}
	authority, owned := coordination.FromContext(ctx)
	var advertised coordination.ExecutionScope
	var run TaskRun
	var target TargetTaskDefinitionPin
	owner := authority.TargetDeviceID
	if authority.Coordinated {
		owner = authority.CoreID
	}
	if !owned || owner == "" || authority.ResourceOwnerID != owner || authority.RoleOwnerID != owner || call == nil || json.Unmarshal(dispatch.OwnedExecutionScope, &advertised) != nil || advertised != authority || len(dispatch.RootTaskMetadata) > 64<<10 || json.Unmarshal(dispatch.RootTaskMetadata, &run) != nil || json.Unmarshal(dispatch.TargetDefinitionPin, &target) != nil || dispatch.TaskGeneration < 1 || run.Generation != dispatch.TaskGeneration || run.TaskRunID != dispatch.TaskRunID || run.TaskDefinitionID != dispatch.TaskDefinitionID || run.ExecutionAttemptID.String() != dispatch.AttemptID || run.ScopeSnapshotID == "" || len(run.Input) != 0 && string(run.Input) != "null" || hashBytes(dispatch.Input) != run.InputHash || !validTaskFingerprint(run.DefinitionFingerprint) || run.ExtensionID != target.ExtensionID || run.ModuleID != target.ModuleID || authority.TargetDeviceID != dispatch.DeviceID.String() || dispatch.AuthorityCallID == "" || dispatch.LeaseID == "" || len(dispatch.Input) > 1<<20 || !json.Valid(dispatch.Input) {
		return empty, NewTaskError(ErrTaskScopeDenied, "设备受托任务缺少完整数据归属授权和原任务元数据")
	}
	if run.ExecutionTarget.DeviceID != dispatch.DeviceID || run.ExecutionTarget.RuntimeID != dispatch.RuntimeID || run.ExecutionTarget.SpaceID.String() != authority.CoreID || run.ExecutionTarget.RuntimeSessionID != dispatch.RuntimeSessionID || run.ExecutionTarget.ConnectionGeneration != dispatch.ConnectionGeneration {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备受托任务的执行连接不一致")
	}
	if dispatch.ProgressBase < 0 || dispatch.ProgressBase > 9007199254740991 {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度基础版本无效")
	}
	guarded := coordination.WithAdditionalGuard(ctx, func(current context.Context) error {
		actual, err := s.DescribeInstalledTask(coordination.WithoutAdditionalGuard(current), dispatch.TaskDefinitionID, dispatch.DeviceID.String())
		if err != nil {
			return err
		}
		if actual != target {
			return NewTaskError(ErrTaskDefinitionInvalid, "设备已安装任务版本与派发版本不一致")
		}
		return nil
	})
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return empty, err
	}
	definition, err := s.store.GetTaskDefinition(guarded, dispatch.TaskDefinitionID)
	if err != nil {
		return empty, err
	}
	var checkpoint json.RawMessage
	if run.CheckpointID != nil {
		var confirmed TaskCheckpoint
		if len(dispatch.ResumeCheckpoint) > (1<<20)+(64<<10) || json.Unmarshal(dispatch.ResumeCheckpoint, &confirmed) != nil || confirmed.CheckpointID != *run.CheckpointID || confirmed.TaskRunID != run.TaskRunID || confirmed.InputHash != run.InputHash || confirmed.Version < 1 || hashBytes(confirmed.Payload) != confirmed.PayloadHash || len(confirmed.Payload) > 1<<20 {
			return empty, NewTaskError(ErrTaskCheckpointIncompatible, "恢复设备任务缺少已确认检查点正文")
		}
		var cursor struct {
			Cursor int64 `json:"cursor"`
		}
		if json.Unmarshal(confirmed.Payload, &cursor) != nil || cursor.Cursor != confirmed.Version {
			return empty, NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点游标与已确认版本不一致")
		}
		checkpoint = confirmed.Payload
	} else if len(dispatch.ResumeCheckpoint) != 0 {
		return empty, NewTaskError(ErrTaskCheckpointIncompatible, "新设备任务不能携带未绑定检查点")
	}
	entry, err := s.config.EntryResolver(guarded, definition)
	if err != nil {
		return empty, err
	}
	node, err := s.config.NodeEnvironmentResolver.Resolve(guarded)
	if err != nil {
		return empty, err
	}
	artifact, err := s.config.HostArtifactResolver.Resolve(guarded, script_host.KindTaskHost)
	if err != nil {
		return empty, err
	}
	workspaceKey := "source-" + uuid.NewString()
	workspace, err := s.createTaskWorkspace(workspaceKey)
	if err != nil {
		return empty, err
	}
	defer s.cleanupWorkspace(workspaceKey, workspace)
	host, err := NewTaskProcessHost(ProcessHostConfig{Generation: run.Generation, InstanceID: uuid.NewString(), TaskRunID: run.TaskRunID, ExtensionID: run.ExtensionID, ModuleID: run.ModuleID, DefHash: definition.DefinitionHash, NodePath: node.NodeBinary, HostPath: artifact.EntryPath, EntryPath: entry, EntryHash: definition.EntryHash, WorkDir: workspace})
	if err != nil {
		return empty, err
	}
	processCtx, stop := context.WithCancelCause(guarded)
	defer stop(nil)
	var mu sync.Mutex
	var outcome protocol.OwnedTaskExecutionOutcome
	var confirmed bool
	var callbackErr error
	recordError := func(err error) {
		mu.Lock()
		if callbackErr == nil {
			callbackErr = err
		}
		mu.Unlock()
		stop(err)
	}
	callbacks := ProcessCallbacks{
		OnRequest: func(current context.Context, id, method string, params json.RawMessage) (json.RawMessage, error) {
			return call(current, id, method, params)
		},
		OnCheckpointConfirmed: func(version int64, payload json.RawMessage, _ string) error {
			params, err := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "version": version, "payload": payload})
			if err != nil {
				return err
			}
			result, err := call(processCtx, "checkpoint/"+uuid.NewString(), "task.checkpoint.save", params)
			var ack struct {
				Version int64 `json:"version"`
			}
			if err == nil && (json.Unmarshal(result, &ack) != nil || ack.Version != version) {
				err = fmt.Errorf("Core 未确认任务检查点版本")
			}
			return err
		},
		OnProgress: func(seq int64, current, total, percentage *float64, stage, message string) {
			if seq < 1 || seq > 9007199254740991-dispatch.ProgressBase {
				recordError(NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度版本已超限"))
				return
			}
			params, err := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "sequence": seq + dispatch.ProgressBase, "current": current, "total": total, "percentage": percentage, "stage": stage, "message": message})
			if err == nil {
				var result json.RawMessage
				result, err = call(processCtx, "progress/"+uuid.NewString(), "task.progress.save", params)
				var ack struct {
					Confirmed bool `json:"confirmed"`
				}
				if err == nil && (json.Unmarshal(result, &ack) != nil || !ack.Confirmed) {
					err = fmt.Errorf("Core 未确认任务进度")
				}
			}
			if err != nil {
				recordError(err)
			}
		},
		OnFinished: func(status string, result json.RawMessage, artifactID, _, message string) {
			if status != "succeeded" {
				recordError(fmt.Errorf("设备任务执行失败：%s", message))
				return
			}
			if artifactID == "" && (!json.Valid(result) || len(result) > 64<<10) || artifactID != "" && (len(result) != 0 || !strings.HasPrefix(artifactID, "artifact-") || !validTaskFingerprint(strings.TrimPrefix(artifactID, "artifact-"))) {
				recordError(NewTaskError(ErrTaskScopeDenied, "设备任务结果或所有者产物引用无效"))
				return
			}
			mu.Lock()
			outcome = protocol.OwnedTaskExecutionOutcome{Result: append(json.RawMessage(nil), result...), ResultArtifactID: artifactID}
			confirmed = true
			mu.Unlock()
		},
	}
	if err := host.Start(processCtx, dispatch.Input, checkpoint, dispatch.DeadlineAt, run.Attempt, run.MaxAttempts, callbacks); err != nil {
		return empty, err
	}
	code, err := host.Wait()
	mu.Lock()
	defer mu.Unlock()
	if callbackErr != nil {
		return empty, callbackErr
	}
	if err != nil || code != 0 || !confirmed {
		return empty, NewTaskError(ErrTaskRuntimeStartFailed, "设备任务进程结果尚未确认")
	}
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return empty, err
	}
	return outcome, nil
}
