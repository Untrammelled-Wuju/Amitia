package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskOutcomePort interface {
	SaveOutcome(context.Context, *TaskRun, string, *TaskRunResult, string, string) error
	Result(context.Context, *TaskRun, *TaskRunResult) (*TaskRunResult, error)
}

type ownedTaskOutcome struct {
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
	document := ownedTaskOutcome{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID, Status: status, ErrorCode: errorCode, ErrorMessage: errorMessage}
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
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	id := taskOutcomeID(run)
	commitScope := scope
	commitScope.RequestID += "|" + id
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: dependencies, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: encoded}}})
	if err != nil {
		return err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 {
		return errors.New("任务结果所有者尚未确认保存，终态不能报告完成")
	}
	return coordination.ValidateCurrent(ctx)
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
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint || document.TaskRunID != run.TaskRunID || document.Generation != run.Generation || document.AttemptID != run.ExecutionAttemptID || document.Status != "succeeded" || document.Result == nil {
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
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return &result, nil
}
