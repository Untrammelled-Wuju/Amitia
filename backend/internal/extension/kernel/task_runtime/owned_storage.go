package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskStoragePort interface {
	Call(context.Context, *TaskRun, string, string, json.RawMessage) (json.RawMessage, error)
}

type AcknowledgedTaskStoragePort struct {
	Data coordination.DataPort
}

type ownedTaskStorage struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	TaskRunID             string                      `json:"taskRunId"`
	Values                map[string]json.RawMessage  `json:"values"`
}

func (p AcknowledgedTaskStoragePort) Call(ctx context.Context, run *TaskRun, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || run.Generation < 1 || run.ExecutionAttemptID == "" || len(requestID) == 0 || len(requestID) > 256 || len(params) > 80<<10 || method != "task.storage.get" && method != "task.storage.set" && method != "task.storage.delete" {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务存储接口、执行身份或请求无效")
	}
	var input struct {
		TaskRunID string          `json:"task_run_id"`
		Key       string          `json:"key"`
		Value     json.RawMessage `json:"value"`
	}
	if json.Unmarshal(params, &input) != nil || input.TaskRunID != run.TaskRunID || len(input.Key) == 0 || len(input.Key) > 128 || strings.ContainsRune(input.Key, '\x00') || method == "task.storage.set" && (!json.Valid(input.Value) || len(input.Value) > 64<<10) {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务存储键值或归属无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return nil, err
	}
	id := "task/storage/" + run.TaskRunID
	resource, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return nil, err
	}
	document := ownedTaskStorage{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, TaskRunID: run.TaskRunID, Values: make(map[string]json.RawMessage)}
	version := int64(0)
	if resource != nil {
		if resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.ID != id || resource.Kind != "checkpoint" || resource.Revision < 1 || len(resource.Body) > 320<<10 || json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint || document.TaskRunID != run.TaskRunID || len(document.Values) > 64 {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务存储与原授权或任务定义不一致")
		}
		version = resource.Revision
	}
	if document.Values == nil {
		document.Values = make(map[string]json.RawMessage)
	}
	if method == "task.storage.get" {
		value := document.Values[input.Key]
		if len(value) == 0 {
			value = json.RawMessage("null")
		}
		if len(value) > 64<<10 || !json.Valid(value) {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务存储值无效")
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]json.RawMessage{"value": value})
	}
	if method == "task.storage.set" {
		document.Values[input.Key] = input.Value
	} else {
		delete(document.Values, input.Key)
	}
	if len(document.Values) > 64 {
		return nil, coordination.ErrPendingLimit
	}
	values, err := json.Marshal(document.Values)
	if err != nil || len(values) > 256<<10 {
		return nil, coordination.ErrPendingLimit
	}
	body, err := json.Marshal(document)
	if err != nil || len(body) > 320<<10 {
		return nil, coordination.ErrPendingLimit
	}
	commitScope := scope
	commitScope.RequestID += "|task-storage|" + run.TaskRunID + "|" + run.ExecutionAttemptID.String() + "|" + hashBytes([]byte(requestID))
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, ExpectedRevision: version, Body: body}}})
	if err != nil {
		return nil, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != version+1 {
		return nil, errors.New("任务存储所有者尚未确认保存")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return json.RawMessage(`{}`), nil
}
