package task_runtime

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskInputPort interface {
	SaveInput(context.Context, *TaskRun) error
	Input(context.Context, *TaskRun) (json.RawMessage, error)
}

type ownedTaskInput struct {
	TaskRunID             string                      `json:"taskRunId"`
	TaskDefinitionID      string                      `json:"taskDefinitionId"`
	ExtensionID           string                      `json:"extensionId"`
	ModuleID              string                      `json:"moduleId"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	InputHash             string                      `json:"inputHash"`
	Input                 []byte                      `json:"inputBytes"`
	Scope                 coordination.ExecutionScope `json:"executionScope"`
}

type AcknowledgedTaskInputPort struct {
	Data coordination.DataPort
}

func taskInputScope(ctx context.Context, run *TaskRun) (coordination.ExecutionScope, error) {
	scope, ok := coordination.FromContext(ctx)
	if !ok || run == nil || run.TaskRunID == "" || len(run.TaskRunID) > 256 || run.ScopeSnapshotID == "" || run.DefinitionFingerprint == "" || run.InputHash == "" || scope.CoreID == "" || scope.AuthorizationRealm != scope.CoreID || scope.InitiatorDeviceID == "" || scope.TargetDeviceID == "" || scope.RequestID == "" || scope.ExecutionID == "" || scope.TurnID == "" || scope.RoleID == "" || scope.RoleRevision < 1 || scope.ProviderEpoch < 1 || scope.TargetProviderEpoch < 1 || scope.ModeRevision < 1 || scope.PermissionRevision < 1 || scope.TargetPermissionRevision < 1 || scope.RoleOwnerID != scope.ResourceOwnerID || scope.ResourceOwnerID == "" {
		return coordination.ExecutionScope{}, NewTaskError(ErrTaskScopeDenied, "任务输入缺少完整所有者和持久授权")
	}
	if scope.Coordinated && scope.ResourceOwnerID != scope.CoreID || !scope.Coordinated && scope.ResourceOwnerID != scope.TargetDeviceID {
		return coordination.ExecutionScope{}, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return coordination.ExecutionScope{}, err
	}
	return scope, nil
}

func (p AcknowledgedTaskInputPort) SaveInput(ctx context.Context, run *TaskRun) error {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return err
	}
	if p.Data == nil || len(run.Input) > 1<<20 || !json.Valid(run.Input) || hashBytes(run.Input) != run.InputHash {
		return NewTaskError(ErrTaskScopeDenied, "任务所有者输入端口或输入完整性无效")
	}
	id := "task/input/" + run.TaskRunID
	document := ownedTaskInput{TaskRunID: run.TaskRunID, TaskDefinitionID: run.TaskDefinitionID, ExtensionID: run.ExtensionID, ModuleID: run.ModuleID, DefinitionFingerprint: run.DefinitionFingerprint, InputHash: run.InputHash, Input: run.Input, Scope: scope}
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	commitScope := scope
	commitScope.RequestID += "|task-input|" + run.TaskRunID
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: encoded}}})
	if err != nil {
		return err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 {
		return errors.New("任务输入所有者尚未确认保存，拒绝入队")
	}
	return coordination.ValidateCurrent(ctx)
}

func (p AcknowledgedTaskInputPort) Input(ctx context.Context, run *TaskRun) (json.RawMessage, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者输入读取端口不可用")
	}
	id := "task/input/" + run.TaskRunID
	resource, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.ID != id || resource.Kind != "checkpoint" || resource.RoleID != scope.RoleID || resource.Revision != 1 || len(resource.Body) > (2<<20)+(64<<10) {
		return nil, coordination.ErrWrongOwner
	}
	var document ownedTaskInput
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.TaskRunID != run.TaskRunID || document.TaskDefinitionID != run.TaskDefinitionID || document.ExtensionID != run.ExtensionID || document.ModuleID != run.ModuleID || document.DefinitionFingerprint != run.DefinitionFingerprint || document.InputHash != run.InputHash || len(document.Input) > 1<<20 || !json.Valid(document.Input) || hashBytes(document.Input) != run.InputHash {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者输入与原授权或定义不一致")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), document.Input...), nil
}
