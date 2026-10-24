package task_runtime

import (
	"encoding/json"
)

func verifiedSourceResumeCheckpoint(run *TaskRun, definition *TaskDefinition, raw json.RawMessage) (json.RawMessage, error) {
	if run == nil || definition == nil {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "恢复缺少原任务和定义")
	}
	if run.CheckpointID == nil {
		if len(raw) != 0 {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "新设备任务不能携带未绑定检查点")
		}
		return nil, nil
	}
	if len(raw) == 0 || len(raw) > (1<<20)+(64<<10) {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "恢复设备任务缺少已确认检查点正文")
	}
	var checkpoint TaskCheckpoint
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "恢复设备检查点正文不是有效JSON")
	}
	if err := validateRecoveryCheckpoint(run, definition, &checkpoint, true); err != nil {
		return nil, err
	}
	var cursor struct {
		Cursor int64 `json:"cursor"`
	}
	if err := json.Unmarshal(checkpoint.Payload, &cursor); err != nil || cursor.Cursor != checkpoint.Version {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点游标与已确认版本不一致")
	}
	return append(json.RawMessage(nil), checkpoint.Payload...), nil
}
