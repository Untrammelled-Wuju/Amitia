package task_runtime

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskProgressPort interface {
	SaveProgress(context.Context, *TaskRun, TaskRunProgress) (TaskRunProgress, error)
	Progress(context.Context, *TaskRun, *TaskRunProgress) (*TaskRunProgress, error)
}

type ownedProgressDocument struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	Generation            int64                       `json:"generation"`
	AttemptID             TaskExecutionAttemptID      `json:"attemptId"`
	Sequence              int64                       `json:"sequence"`
	Payload               []byte                      `json:"progressBytes"`
}

type ownedProgressReference struct {
	ResourceID string `json:"resourceId"`
	Revision   int64  `json:"revision"`
	Hash       string `json:"hash"`
}

type AcknowledgedTaskProgressPort struct {
	Data coordination.DataPort
}

func taskProgressID(run *TaskRun) string {
	return "task/progress/" + run.TaskRunID + "/" + strconv.FormatInt(run.Generation, 10) + "/" + run.ExecutionAttemptID.String()
}

func (p AcknowledgedTaskProgressPort) SaveProgress(ctx context.Context, run *TaskRun, progress TaskRunProgress) (TaskRunProgress, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return TaskRunProgress{}, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || run.Generation < 1 || run.ExecutionAttemptID == "" || progress.TaskRunID != run.TaskRunID || progress.Sequence < 1 || len(progress.Stage) > 512 || len(progress.Message) > 32<<10 {
		return TaskRunProgress{}, NewTaskError(ErrTaskScopeDenied, "任务进度所有者端口或参数无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return TaskRunProgress{}, err
	}
	encoded, err := json.Marshal(progress)
	if err != nil || len(encoded) > 64<<10 {
		return TaskRunProgress{}, NewTaskError(ErrTaskScopeDenied, "任务进度正文格式无效或超过上限")
	}
	id := taskProgressID(run)
	current, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return TaskRunProgress{}, err
	}
	version := int64(0)
	if current != nil {
		var previous ownedProgressDocument
		if current.Deleted || current.OwnerID != scope.ResourceOwnerID || current.RoleID != scope.RoleID || current.ID != id || current.Kind != "checkpoint" || json.Unmarshal(current.Body, &previous) != nil || previous.Scope != scope || previous.DefinitionFingerprint != run.DefinitionFingerprint || previous.Generation != run.Generation || previous.AttemptID != run.ExecutionAttemptID || previous.Sequence >= progress.Sequence {
			return TaskRunProgress{}, coordination.ErrResourceVersion
		}
		version = current.Revision
	}
	document := ownedProgressDocument{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, Generation: run.Generation, AttemptID: run.ExecutionAttemptID, Sequence: progress.Sequence, Payload: encoded}
	body, err := json.Marshal(document)
	if err != nil {
		return TaskRunProgress{}, err
	}
	commitScope := scope
	commitScope.RequestID += "|" + id + "|" + strconv.FormatInt(progress.Sequence, 10)
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, ExpectedRevision: version, Body: body}}})
	if err != nil {
		return TaskRunProgress{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != version+1 {
		return TaskRunProgress{}, NewTaskError(ErrTaskScopeDenied, "任务进度所有者尚未确认保存")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return TaskRunProgress{}, err
	}
	reference, _ := json.Marshal(ownedProgressReference{ResourceID: id, Revision: version + 1, Hash: hashBytes(encoded)})
	return TaskRunProgress{TaskRunID: progress.TaskRunID, Sequence: progress.Sequence, UpdatedAt: progress.UpdatedAt, Details: reference}, nil
}

func (p AcknowledgedTaskProgressPort) Progress(ctx context.Context, run *TaskRun, metadata *TaskRunProgress) (*TaskRunProgress, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	var reference ownedProgressReference
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || metadata == nil || metadata.TaskRunID != run.TaskRunID || json.Unmarshal(metadata.Details, &reference) != nil || reference.ResourceID != taskProgressID(run) || reference.Revision < 1 || reference.Hash == "" {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务进度引用或所有者端口无效")
	}
	resource, err := port.Resource(ctx, scope, "checkpoint", reference.ResourceID)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.ID != reference.ResourceID || resource.Kind != "checkpoint" || resource.Revision != reference.Revision || len(resource.Body) > 192<<10 {
		return nil, coordination.ErrResourceVersion
	}
	var document ownedProgressDocument
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint || document.Generation != run.Generation || document.AttemptID != run.ExecutionAttemptID || document.Sequence != metadata.Sequence || len(document.Payload) > 64<<10 || hashBytes(document.Payload) != reference.Hash {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务进度与原授权或确认引用不一致")
	}
	var progress TaskRunProgress
	if json.Unmarshal(document.Payload, &progress) != nil || progress.TaskRunID != run.TaskRunID || progress.Sequence != metadata.Sequence {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务进度正文无效")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return &progress, nil
}
