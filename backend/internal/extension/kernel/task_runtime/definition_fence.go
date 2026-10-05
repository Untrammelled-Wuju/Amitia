package task_runtime

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *TaskRuntimeService) guardOwnedTaskDefinition(ctx context.Context, run *TaskRun) context.Context {
	identity := CloneTaskRun(run)
	return coordination.WithAdditionalGuard(ctx, func(current context.Context) error {
		definition, err := s.store.GetTaskDefinition(current, identity.TaskDefinitionID)
		if err != nil {
			return err
		}
		if err := validateTaskDefinition(true, identity, definition); err != nil {
			return err
		}
		if s.config.InstalledDefinitionValidator != nil {
			return s.config.InstalledDefinitionValidator(current, definition)
		}
		return nil
	})
}

func taskDefinitionFingerprint(def *TaskDefinition) (string, error) {
	if def == nil {
		return "", NewTaskError(ErrTaskDefinitionInvalid, "任务定义不存在")
	}
	encoded, err := json.Marshal(def)
	if err != nil {
		return "", WrapTaskError(ErrTaskDefinitionInvalid, "任务定义格式无效", err)
	}
	return hashBytes(encoded), nil
}

func validateTaskDefinition(ctxOwned bool, run *TaskRun, def *TaskDefinition) error {
	if run == nil || def == nil || run.TaskDefinitionID != def.TaskID || run.ExtensionID != def.ExtensionID || run.ModuleID != def.ModuleID {
		return NewTaskError(ErrTaskDefinitionInvalid, "任务定义身份已变化")
	}
	if run.DefinitionFingerprint == "" {
		if ctxOwned {
			return NewTaskError(ErrTaskDefinitionInvalid, "设备任务缺少入队时的定义指纹")
		}
		return nil
	}
	actual, err := taskDefinitionFingerprint(def)
	if err != nil {
		return err
	}
	if actual != run.DefinitionFingerprint {
		return NewTaskError(ErrTaskDefinitionInvalid, "任务定义已更新，请确认后重新创建任务")
	}
	return nil
}
