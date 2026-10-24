package task_runtime

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedTaskCommitCursor struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	TaskRunID             string                      `json:"taskRunId"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	Generation            int64                       `json:"generation"`
	AttemptID             string                      `json:"attemptId"`
	CheckpointVersion     int64                       `json:"checkpointVersion"`
	CheckpointID          string                      `json:"checkpointId,omitempty"`
	Terminal              bool                        `json:"terminal"`
	OutcomeID             string                      `json:"outcomeId,omitempty"`
	InFlightEffects       map[string]string           `json:"inFlightEffects,omitempty"`
}

func taskOwnerCommitCursorID(run *TaskRun) string {
	return "task/commit-cursor/" + run.TaskRunID
}

func readTaskOwnerCommitCursor(ctx context.Context, data coordination.DataPort, scope coordination.ExecutionScope, run *TaskRun) (*ownedTaskCommitCursor, int64, error) {
	reader, ok := data.(coordination.ResourcePort)
	if !ok {
		return nil, 0, NewTaskError(ErrTaskDependencyUnavailable, "Owner提交游标读取端口不可用")
	}
	id := taskOwnerCommitCursorID(run)
	resource, err := reader.Resource(ctx, scope, "checkpoint", id)
	if err != nil || resource == nil {
		return nil, 0, err
	}
	if resource.Deleted || resource.Kind != "checkpoint" || resource.ID != id ||
		resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID ||
		resource.Revision < 1 || len(resource.Body) > 32<<10 {
		return nil, 0, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner提交游标所有权或版本无效")
	}
	var cursor ownedTaskCommitCursor
	if err := json.Unmarshal(resource.Body, &cursor); err != nil ||
		cursor.Scope != scope || cursor.TaskRunID != run.TaskRunID ||
		cursor.DefinitionFingerprint != run.DefinitionFingerprint || cursor.Generation < 1 ||
		cursor.Generation > run.Generation || cursor.AttemptID == "" ||
		(cursor.Generation == run.Generation && cursor.AttemptID != run.ExecutionAttemptID.String()) ||
		cursor.CheckpointVersion < 0 ||
		(cursor.CheckpointVersion == 0) != (cursor.CheckpointID == "") ||
		(cursor.Terminal != (cursor.OutcomeID != "")) {
		return nil, 0, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner提交游标内容与执行身份不一致")
	}
	if len(cursor.InFlightEffects) > 64 || (cursor.Terminal && len(cursor.InFlightEffects) != 0) {
		return nil, 0, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner提交游标存在无效的未完成工具效果")
	}
	for effectID, fingerprint := range cursor.InFlightEffects {
		if len(effectID) < len("task/host/") || effectID[:len("task/host/")] != "task/host/" ||
			len(effectID) > 320 || len(fingerprint) != 64 {
			return nil, 0, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner提交游标工具效果身份损坏")
		}
	}
	if cursor.Terminal && cursor.OutcomeID != taskOutcomeIDForIdentity(cursor.TaskRunID, cursor.Generation, cursor.AttemptID) {
		return nil, 0, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner提交游标终态记录不匹配")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, 0, err
	}
	return &cursor, resource.Revision, nil
}

func taskOutcomeIDForIdentity(taskRunID string, generation int64, attemptID string) string {
	return "task/outcome/" + taskRunID + "/" + strconv.FormatInt(generation, 10) + "/" + attemptID
}

func ownerCursorMutation(scope coordination.ExecutionScope, run *TaskRun, previous *ownedTaskCommitCursor, revision int64, checkpoint *TaskCheckpoint, terminal bool) (coordination.Mutation, error) {
	if previous != nil && previous.Terminal {
		return coordination.Mutation{}, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner任务已提交终态，禁止再次推进")
	}
	cursor := ownedTaskCommitCursor{
		Scope: scope, TaskRunID: run.TaskRunID, DefinitionFingerprint: run.DefinitionFingerprint,
		Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(),
	}
	if previous != nil {
		cursor.CheckpointVersion, cursor.CheckpointID = previous.CheckpointVersion, previous.CheckpointID
		cursor.InFlightEffects = previous.InFlightEffects
	}
	if (checkpoint != nil || terminal) && len(cursor.InFlightEffects) != 0 {
		return coordination.Mutation{}, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner存在未确认的Native工具执行，禁止提交检查点或终态")
	}
	if checkpoint != nil {
		if !ownerInitialCheckpointAllowed(previous != nil, run.CheckpointID != nil, checkpoint.Version) {
			return coordination.Mutation{}, NewTaskError(ErrTaskCheckpointIncompatible, "Owner初始提交缺少权威游标或前序检查点，禁止跳版本写入")
		}
		if previous != nil && (checkpoint.Version != previous.CheckpointVersion+1 ||
			(previous.CheckpointVersion > 0 && (run.CheckpointID == nil || *run.CheckpointID != previous.CheckpointID))) {
			return coordination.Mutation{}, NewTaskError(ErrTaskCheckpointIncompatible, "检查点不满足Owner单调提交游标")
		}
		cursor.CheckpointVersion, cursor.CheckpointID = checkpoint.Version, checkpoint.CheckpointID
	}
	if terminal {

		if previous != nil && previous.CheckpointVersion > 0 &&
			(run.CheckpointID == nil || *run.CheckpointID != previous.CheckpointID) {
			return coordination.Mutation{}, NewTaskError(ErrTaskCheckpointIncompatible, "Owner终态不能越过未投影的检查点")
		}
		cursor.Terminal, cursor.OutcomeID = true, taskOutcomeID(run)
	}
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return coordination.Mutation{}, err
	}
	return coordination.Mutation{Kind: "checkpoint", ID: taskOwnerCommitCursorID(run), RoleID: scope.RoleID, ExpectedRevision: revision, Body: encoded}, nil
}

func ownerCursorNativeEffectMutation(scope coordination.ExecutionScope, run *TaskRun, previous *ownedTaskCommitCursor, revision int64, operationID, inputHash string, confirmed bool) (coordination.Mutation, error) {
	mutation, err := ownerCursorMutation(scope, run, previous, revision, nil, false)
	if err != nil {
		return coordination.Mutation{}, err
	}
	var cursor ownedTaskCommitCursor
	if err := json.Unmarshal(mutation.Body, &cursor); err != nil {
		return coordination.Mutation{}, err
	}
	if previous != nil && len(previous.InFlightEffects) != 0 &&
		(previous.Generation != run.Generation || previous.AttemptID != run.ExecutionAttemptID.String()) {
		return coordination.Mutation{}, NewTaskError(ErrTaskExecutionAttemptInvalid, "旧执行代次尚有未确认的Native效果，必须先对账")
	}
	if previous != nil && previous.CheckpointVersion > 0 &&
		(run.CheckpointID == nil || *run.CheckpointID != previous.CheckpointID) {
		return coordination.Mutation{}, NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点未投影，禁止开始或确认新工具效果")
	}
	if cursor.InFlightEffects == nil {
		cursor.InFlightEffects = make(map[string]string)
	}
	existing, present := cursor.InFlightEffects[operationID]
	if confirmed {
		if !present || existing != inputHash {
			return coordination.Mutation{}, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner工具效果提交缺少原started凭证")
		}
		delete(cursor.InFlightEffects, operationID)
	} else {
		if present || len(cursor.InFlightEffects) >= 64 {
			return coordination.Mutation{}, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner工具效果已开始或达到并发上限")
		}
		cursor.InFlightEffects[operationID] = inputHash
	}
	mutation.Body, err = json.Marshal(cursor)
	if err != nil {
		return coordination.Mutation{}, err
	}
	return mutation, nil
}
