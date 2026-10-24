package task_runtime

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func taskMayCallNativeEffects(definition *TaskDefinition) (bool, error) {
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil {
		return false, err
	}
	for _, requirement := range requirements {
		if requirement.PermissionID == "service.tool.execute" || requirement.PermissionID == "event.emit" {
			return true, nil
		}
	}
	return false, nil
}

func (p AcknowledgedTaskHostPort) InspectPendingNativeEffects(ctx context.Context, run *TaskRun) ([]string, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return nil, NewTaskError(ErrTaskDependencyUnavailable, "Owner未提供工具效果只读核查端口")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	cursor, _, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return nil, err
	}

	if cursor == nil {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner缺少Native提交游标，无法证明历史工具效果均已对账")
	}

	if cursor.Terminal {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner游标已标记终态，但原终态对账未完成")
	}
	ids := make([]string, 0, len(cursor.InFlightEffects))
	for id, fingerprint := range cursor.InFlightEffects {
		stored, err := resources.Resource(ctx, scope, "tool-result", id)
		if err != nil {
			return nil, err
		}
		var operation ownedTaskHostOperation
		if stored == nil || stored.Deleted || stored.Kind != "tool-result" || stored.ID != id ||
			stored.OwnerID != scope.ResourceOwnerID || stored.RoleID != scope.RoleID || stored.Revision != 1 ||
			len(stored.Body) > 128<<10 || json.Unmarshal(stored.Body, &operation) != nil ||
			operation.Scope != scope || operation.TaskRunID != run.TaskRunID ||
			operation.DefinitionFingerprint != run.DefinitionFingerprint ||
			operation.Generation != cursor.Generation || operation.AttemptID != cursor.AttemptID ||
			operation.InputHash != fingerprint || operation.State != "started" ||
			operation.RequestID == "" ||
			(operation.Method != "task.host.executeTool" && operation.Method != "task.host.emitEvent") ||
			TaskHostNativeOperationID(&TaskRun{
				TaskRunID:          run.TaskRunID,
				ExecutionAttemptID: TaskExecutionAttemptID(cursor.AttemptID),
			}, operation.RequestID, TaskHostNativeCall{EffectID: operation.EffectID}) != id ||
			len(operation.ResultBytes) != 0 || operation.ResultHash != "" {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner待对账工具效果与原始started记录不一致")
		}
		ids = append(ids, id)
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return ids, nil
}
