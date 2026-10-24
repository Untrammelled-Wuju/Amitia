package task_runtime

import "context"

type pendingOwnerCheckpointInspector interface {
	InspectPendingCheckpoint(context.Context, *TaskRun, *TaskDefinition, *TaskCheckpoint) (*TaskCheckpoint, error)
}

func verifyOwnerRecoveryQueuePosition(ctx context.Context, inspector pendingOwnerCheckpointInspector, run *TaskRun, def *TaskDefinition, checkpoint *TaskCheckpoint) error {
	if inspector == nil || run == nil || def == nil || checkpoint == nil || run.CheckpointID == nil || *run.CheckpointID != checkpoint.CheckpointID {
		return NewTaskError(ErrTaskCheckpointIncompatible, "Owner恢复最终入队缺少已确认检查点")
	}
	newer, err := inspector.InspectPendingCheckpoint(ctx, run, def, checkpoint)
	if err != nil {
		return err
	}
	if newer != nil {
		return NewTaskError(ErrTaskCheckpointIncompatible, "Owner在恢复入队前已经推进检查点，禁止启动过期执行代次")
	}
	return nil
}
