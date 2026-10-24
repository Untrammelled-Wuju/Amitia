package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskOutcomePort interface {
	SaveOutcome(context.Context, *TaskRun, string, *TaskRunResult, string, string) error
	Result(context.Context, *TaskRun, *TaskRunResult) (*TaskRunResult, error)
}

type ownedTaskOutcome struct {
	CommitProtocolVersion int                         `json:"commitProtocolVersion,omitempty"`
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	TaskRunID             string                      `json:"taskRunId"`
	Generation            int64                       `json:"generation"`
	AttemptID             TaskExecutionAttemptID      `json:"attemptId"`
	Status                string                      `json:"status"`
	ErrorCode             string                      `json:"errorCode,omitempty"`
	ErrorMessage          string                      `json:"errorMessage,omitempty"`
	Result                *TaskRunResult              `json:"result,omitempty"`
	Output                []byte                      `json:"outputBytes,omitempty"`
}

type AcknowledgedTaskOutcomePort struct {
	Data coordination.DataPort
}

func taskOutcomeID(run *TaskRun) string {
	return "task/outcome/" + run.TaskRunID + "/" + strconv.FormatInt(run.Generation, 10) + "/" + run.ExecutionAttemptID.String()
}

func (p AcknowledgedTaskOutcomePort) SaveOutcome(ctx context.Context, run *TaskRun, status string, result *TaskRunResult, errorCode, errorMessage string) error {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return err
	}
	if p.Data == nil || run.Generation < 1 || run.ExecutionAttemptID == "" || len(run.ExecutionAttemptID) > 128 || status != "succeeded" && status != "failed" && status != "cancelled" && status != "timed_out" && status != "manual_intervention" || len(errorCode) > 8<<10 || len(errorMessage) > 8<<10 {
		return NewTaskError(ErrTaskScopeDenied, "任务终态参数或所有者端口无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return err
	}
	document := ownedTaskOutcome{CommitProtocolVersion: 1, Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID, Status: status, ErrorCode: errorCode, ErrorMessage: errorMessage}
	dependencies := []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}
	if status == "succeeded" {
		if result == nil || result.TaskRunID != run.TaskRunID {
			return NewTaskError(ErrTaskScopeDenied, "任务所有者结果无效或需要产物端口")
		}
		switch result.ResultType {
		case ResultInlineJSON:
			if result.ArtifactID != "" || len(result.ResultJSON) > 64<<10 || !json.Valid(result.ResultJSON) || hashBytes(result.ResultJSON) != result.ResultHash {
				return NewTaskError(ErrTaskScopeDenied, "任务所有者结果正文无效")
			}
		case ResultArtifact:
			_, hash, err := (AcknowledgedTaskArtifactPort{Data: p.Data}).Result(ctx, run, result.ArtifactID)
			if err != nil {
				return err
			}
			if len(result.ResultJSON) != 0 || result.ResultHash != hash {
				return NewTaskError(ErrTaskScopeDenied, "任务产物结果确认摘要不一致")
			}
			dependencies = append(dependencies, coordination.ResourceVersion{Kind: "checkpoint", ID: "task/artifact/" + result.ArtifactID, Revision: 1})
		default:
			return NewTaskError(ErrTaskScopeDenied, "任务结果类型无效")
		}
		metadata := *result
		metadata.ResultJSON = nil
		document.Result, document.Output = &metadata, result.ResultJSON
	} else if result != nil {
		return NewTaskError(ErrTaskScopeDenied, "失败或取消任务不能保存成功结果")
	}

	previous, err := p.loadCommittedOutcome(ctx, run)
	if err != nil {
		return err
	}
	if previous != nil {
		if !sameCommittedOutcome(previous, &document) {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者已提交不同的终态结果，禁止覆盖")
		}
		return coordination.ValidateCurrent(ctx)
	}
	if err := p.rejectUnprojectedOwnerCheckpoint(ctx, run, scope); err != nil {
		return err
	}
	previousCursor, cursorRevision, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return err
	}
	cursorMutation, err := ownerCursorMutation(scope, run, previousCursor, cursorRevision, nil, true)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	id := taskOutcomeID(run)
	commitScope := scope
	commitScope.RequestID += "|" + id
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: dependencies, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: encoded}, cursorMutation}})
	if err != nil {
		return err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 || ack.Versions["checkpoint/"+taskOwnerCommitCursorID(run)] != cursorRevision+1 {
		return errors.New("任务结果所有者尚未确认保存，终态不能报告完成")
	}
	return coordination.ValidateCurrent(ctx)
}

func (p AcknowledgedTaskOutcomePort) rejectUnprojectedOwnerCheckpoint(ctx context.Context, run *TaskRun, scope coordination.ExecutionScope) error {
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return NewTaskError(ErrTaskDependencyUnavailable, "所有者检查点读取端口未就绪")
	}
	version := int64(0)
	if run.CheckpointID != nil {
		if *run.CheckpointID == "" {
			return NewTaskError(ErrTaskCheckpointIncompatible, "任务的当前检查点引用无效")
		}
		id := "task/checkpoint/" + *run.CheckpointID
		resource, err := resources.Resource(ctx, scope, "checkpoint", id)
		if err != nil {
			return err
		}
		if resource == nil || resource.Deleted || resource.ID != id || resource.Kind != "checkpoint" ||
			resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Revision != 1 || len(resource.Body) > (2<<20)+(64<<10) {
			return NewTaskError(ErrTaskCheckpointIncompatible, "任务已引用的Owner检查点无法验证")
		}
		var record ownedTaskCheckpoint
		if err := json.Unmarshal(resource.Body, &record); err != nil {
			return NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点记录无法解码")
		}
		confirmed, err := (AcknowledgedTaskCheckpointPort{Data: p.Data}).Checkpoint(ctx, run, &record.Checkpoint)
		if err != nil {
			return err
		}
		if confirmed.Version < 1 || confirmed.Version == math.MaxInt64 {
			return NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点版本无法继续对账")
		}
		version = confirmed.Version
	}
	nextID := "task/checkpoint/" + ownedStableCheckpointID(run, version+1)
	pending, err := resources.Resource(ctx, scope, "checkpoint", nextID)
	if err != nil {
		return err
	}
	if pending != nil {
		return NewTaskError(ErrTaskCheckpointIncompatible, "Owner存在尚未投影的后续检查点，须先对账再提交终态")
	}
	return coordination.ValidateCurrent(ctx)
}

type CommittedTaskOutcome struct {
	Status string
	Result *TaskRunResult
}

func (p AcknowledgedTaskOutcomePort) InspectOutcome(ctx context.Context, run *TaskRun) (*CommittedTaskOutcome, error) {
	document, err := p.loadCommittedOutcome(ctx, run)
	if err != nil || document == nil {
		return nil, err
	}
	outcome := &CommittedTaskOutcome{Status: document.Status}
	if document.Result != nil {
		metadata := *document.Result
		metadata.ResultJSON = nil
		outcome.Result = &metadata
	}
	return outcome, nil
}

func (p AcknowledgedTaskOutcomePort) loadCommittedOutcome(ctx context.Context, run *TaskRun) (*ownedTaskOutcome, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return nil, NewTaskError(ErrTaskDependencyUnavailable, "所有者终态读取端口未就绪")
	}
	if _, err = (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	id := taskOutcomeID(run)
	resource, err := resources.Resource(ctx, scope, "checkpoint", id)
	if err != nil || resource == nil {
		return nil, err
	}
	if resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != "checkpoint" || resource.ID != id || resource.Revision != 1 || len(resource.Body) > 192<<10 {
		return nil, NewTaskError(ErrTaskScopeDenied, "所有者终态记录无效或已被修改")
	}
	var document ownedTaskOutcome
	if json.Unmarshal(resource.Body, &document) != nil || document.CommitProtocolVersion < 0 || document.CommitProtocolVersion > 1 || document.Scope != scope || document.TaskRunID != run.TaskRunID || document.DefinitionFingerprint != run.DefinitionFingerprint || document.Generation != run.Generation || document.AttemptID != run.ExecutionAttemptID || len(document.ErrorCode) > 8<<10 || len(document.ErrorMessage) > 8<<10 {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者终态与原任务身份不一致")
	}
	switch document.Status {
	case "succeeded":
		if document.Result == nil || document.Result.TaskRunID != run.TaskRunID || document.Result.ResultHash == "" {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者成功结果缺少可信元数据")
		}
		switch document.Result.ResultType {
		case ResultInlineJSON:
			if document.Result.ArtifactID != "" || len(document.Result.ResultJSON) != 0 || len(document.Output) > 64<<10 || !json.Valid(document.Output) || hashBytes(document.Output) != document.Result.ResultHash {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者内联结果摘要不一致")
			}
		case ResultArtifact:
			if document.Result.ArtifactID == "" || len(document.Result.ResultJSON) != 0 || len(document.Output) != 0 {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者产物结果格式无效")
			}
			_, hash, artifactErr := (AcknowledgedTaskArtifactPort{Data: p.Data}).Result(ctx, run, document.Result.ArtifactID)
			if artifactErr != nil || hash != document.Result.ResultHash {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者产物无法验证")
			}
		default:
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者结果类型无效")
		}
	case "failed", "cancelled", "timed_out", "manual_intervention":
		if document.Result != nil || len(document.Output) != 0 {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者失败终态包含非法结果")
		}
	default:
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者终态状态未知")
	}
	cursor, _, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return nil, err
	}
	if cursor == nil || !ownerTerminalConfirmed(id, cursor.Terminal, cursor.OutcomeID) {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner终态记录存在但未获提交游标确认，禁止恢复或重试确认")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return &document, nil
}

func sameCommittedOutcome(previous, proposed *ownedTaskOutcome) bool {
	if previous == nil || proposed == nil || previous.Status != proposed.Status || previous.Scope != proposed.Scope || previous.DefinitionFingerprint != proposed.DefinitionFingerprint || previous.TaskRunID != proposed.TaskRunID || previous.Generation != proposed.Generation || previous.AttemptID != proposed.AttemptID || previous.ErrorCode != proposed.ErrorCode || previous.ErrorMessage != proposed.ErrorMessage {
		return false
	}
	if proposed.Status != "succeeded" {
		return previous.Result == nil && proposed.Result == nil && len(previous.Output) == 0 && len(proposed.Output) == 0
	}
	if previous.Result == nil || proposed.Result == nil {
		return false
	}
	return previous.Result.TaskRunID == proposed.Result.TaskRunID && previous.Result.ResultType == proposed.Result.ResultType && previous.Result.ResultHash == proposed.Result.ResultHash && previous.Result.ArtifactID == proposed.Result.ArtifactID && string(previous.Output) == string(proposed.Output)
}

func (p AcknowledgedTaskOutcomePort) Result(ctx context.Context, run *TaskRun, metadata *TaskRunResult) (*TaskRunResult, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || metadata == nil || metadata.TaskRunID != run.TaskRunID {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务结果所有者端口或引用无效")
	}
	id := taskOutcomeID(run)
	resource, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.Kind != "checkpoint" || resource.ID != id || resource.RoleID != scope.RoleID || resource.Revision != 1 || len(resource.Body) > 192<<10 {
		return nil, coordination.ErrWrongOwner
	}
	var document ownedTaskOutcome
	if json.Unmarshal(resource.Body, &document) != nil || document.CommitProtocolVersion < 0 || document.CommitProtocolVersion > 1 || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint || document.TaskRunID != run.TaskRunID || document.Generation != run.Generation || document.AttemptID != run.ExecutionAttemptID || document.Status != "succeeded" || document.Result == nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务结果的原授权或执行身份不一致")
	}
	result := *document.Result
	result.ResultJSON = append(json.RawMessage(nil), document.Output...)
	if result.TaskRunID != metadata.TaskRunID || result.ResultHash != metadata.ResultHash || result.ResultType != metadata.ResultType || result.ArtifactID != metadata.ArtifactID {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务结果正文与确认引用不一致")
	}
	if result.ResultType == ResultArtifact {
		if len(document.Output) != 0 {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务产物结果混入普通结果正文")
		}
		content, hash, err := (AcknowledgedTaskArtifactPort{Data: p.Data}).Result(ctx, run, result.ArtifactID)
		if err != nil {
			return nil, err
		}
		if result.ResultHash != hash {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务产物结果摘要不一致")
		}
		result.ResultJSON = append(json.RawMessage(nil), content...)
	} else if result.ResultType != ResultInlineJSON || result.ArtifactID != "" || len(result.ResultJSON) > 64<<10 || !json.Valid(result.ResultJSON) || hashBytes(result.ResultJSON) != result.ResultHash {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务结果正文与确认引用不一致")
	}
	cursor, _, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return nil, err
	}
	if cursor == nil {
		if document.CommitProtocolVersion != 0 || run.Status != RunStatusSucceeded || run.FinishedAt == nil || run.FinishedAt.IsZero() {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner终态结果尚未获得提交游标确认")
		}
	} else if !ownerTerminalConfirmed(id, cursor.Terminal, cursor.OutcomeID) {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner终态结果尚未获得提交游标确认")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return &result, nil
}
