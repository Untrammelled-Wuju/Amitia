package task_runtime

import (
	"context"
	"encoding/json"
	"math"
	"reflect"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (p AcknowledgedTaskCheckpointPort) InspectPendingCheckpoint(ctx context.Context, run *TaskRun, def *TaskDefinition, previous *TaskCheckpoint) (*TaskCheckpoint, error) {
	if run == nil || def == nil || run.Generation < 1 || run.ExecutionAttemptID == "" {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "待核对检查点缺少原执行身份")
	}
	version := int64(1)
	previousVersion := int64(0)
	previousID := ""
	if previous != nil {
		if previous.Version < 1 || previous.Version == math.MaxInt64 || run.CheckpointID == nil || *run.CheckpointID != previous.CheckpointID {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "待核对检查点的前序版本无效")
		}
		version = previous.Version + 1
		previousVersion = previous.Version
		previousID = previous.CheckpointID
	} else if run.CheckpointID != nil {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "任务检查点引用存在但本地记录缺失")
	}
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return nil, NewTaskError(ErrTaskDependencyUnavailable, "所有者检查点读取端口未就绪")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	id := ownedStableCheckpointID(run, version)
	cursor, cursorRevision, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return nil, err
	}
	required := false
	if cursor != nil {
		var valid bool
		required, valid = ownedPendingCheckpointMatchesCursor(previousVersion, version, previousID, id, cursor.CheckpointVersion, cursor.CheckpointID, cursor.Terminal)
		if !valid {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "Owner提交游标与本地检查点或执行终态不一致，禁止恢复")
		}
	}
	resourceID := "task/checkpoint/" + id
	resource, err := resources.Resource(ctx, scope, "checkpoint", resourceID)
	if err != nil {
		return nil, err
	}
	if resource == nil {
		if required {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "Owner已提交检查点，但检查点正文缺失，禁止跳过并恢复旧版本")
		}
		if err := verifyOwnerPendingCursorStable(ctx, p.Data, scope, run, cursor, cursorRevision); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if cursor == nil || !required {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点尚未进入权威提交游标，禁止采用孤儿检查点")
	}
	if resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != "checkpoint" || resource.ID != resourceID || resource.Revision != 1 || len(resource.Body) > (2<<20)+(64<<10) {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "待核对检查点的所有者记录不可信")
	}
	var record ownedTaskCheckpoint
	if json.Unmarshal(resource.Body, &record) != nil || record.Scope != scope || record.DefinitionFingerprint != run.DefinitionFingerprint ||
		record.ExecutionGeneration != run.Generation || record.ExecutionAttemptID != run.ExecutionAttemptID.String() ||
		record.Checkpoint.CheckpointID != id || record.Checkpoint.Version != version {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "待核对检查点不属于当前执行代次")
	}

	projected := CloneTaskRun(run)
	projected.CheckpointID = &id
	confirmed, err := p.Checkpoint(ctx, projected, &record.Checkpoint)
	if err != nil {
		return nil, err
	}
	if err := validateRecoveryCheckpoint(projected, def, confirmed, true); err != nil {
		return nil, err
	}
	if err := verifyOwnerPendingCursorStable(ctx, p.Data, scope, run, cursor, cursorRevision); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return confirmed, nil
}

func (s *TaskRuntimeService) reconcilePendingOwnerCheckpoint(ctx context.Context, run *TaskRun, def *TaskDefinition, current *TaskCheckpoint) (*TaskRun, *TaskCheckpoint, error) {
	inspector, ok := s.config.OwnedCheckpoints.(interface {
		InspectPendingCheckpoint(context.Context, *TaskRun, *TaskDefinition, *TaskCheckpoint) (*TaskCheckpoint, error)
	})
	if !ok {

		return nil, nil, NewTaskError(ErrTaskDependencyUnavailable, "所有者检查点端口不支持跨进程提交对账，禁止按旧检查点恢复")
	}
	pending, err := inspector.InspectPendingCheckpoint(ctx, run, def, current)
	if err != nil || pending == nil {
		return run, current, err
	}
	initialCheckpoint := *pending
	initialCheckpoint.Payload = append([]byte(nil), pending.Payload...)
	var ownerData coordination.DataPort
	switch port := s.config.OwnedCheckpoints.(type) {
	case AcknowledgedTaskCheckpointPort:
		ownerData = port.Data
	case *AcknowledgedTaskCheckpointPort:
		if port != nil {
			ownerData = port.Data
		}
	}
	var ownerScope coordination.ExecutionScope
	var committedCursor *ownedTaskCommitCursor
	var cursorRevision int64
	if ownerData != nil {
		ownerScope, err = taskInputScope(ctx, run)
		if err != nil {
			return nil, nil, err
		}
		committedCursor, cursorRevision, err = readTaskOwnerCommitCursor(ctx, ownerData, ownerScope, run)
		if err != nil {
			return nil, nil, err
		}
		if committedCursor == nil || committedCursor.Terminal || committedCursor.CheckpointVersion != pending.Version || committedCursor.CheckpointID != pending.CheckpointID || len(committedCursor.InFlightEffects) != 0 {
			return nil, nil, NewTaskError(ErrTaskCheckpointIncompatible, "Owner提交游标不再允许投影待恢复检查点")
		}
	}
	next := CloneTaskRun(run)
	cpID := pending.CheckpointID
	next.CheckpointID = &cpID
	next.Revision = NextRevision(run.Revision)
	metadata := *pending
	metadata.Payload = nil
	if err := s.withManagementTaskTx(ctx, func(txCtx context.Context) error {
		if ownerData != nil {
			if err := verifyOwnerPendingCursorStable(txCtx, ownerData, ownerScope, run, committedCursor, cursorRevision); err != nil {
				return err
			}
		}
		latest, err := inspector.InspectPendingCheckpoint(txCtx, run, def, current)
		if err != nil {
			return err
		}
		if latest == nil || !reflect.DeepEqual(latest, &initialCheckpoint) {
			return NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点在本地事务提交前发生变化，禁止投影")
		}
		ok, err := s.store.UpdateTaskRunCAS(txCtx, next, run.Status, run.Generation, run.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "待对账检查点投影遇到并发任务更新")
		}
		return s.store.PutCheckpoint(txCtx, &metadata)
	}); err != nil {
		return nil, nil, err
	}
	return next, pending, nil
}
