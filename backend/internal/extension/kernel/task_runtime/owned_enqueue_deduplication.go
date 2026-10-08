package task_runtime

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TaskRunCreationStore interface {
	CreateTaskRun(context.Context, *TaskRun) (bool, error)
}

var errOwnedTaskAlreadyCreated = errors.New("任务请求已经创建")

func OwnedRequestTaskRunID(authority coordination.ExecutionScope) (string, error) {
	if authority.CoreID == "" || authority.SpaceID != authority.CoreID || authority.AuthorizationRealm != authority.CoreID || authority.InitiatorDeviceID == "" || authority.RequestID == "" || len(authority.RequestID) > 128 {
		return "", coordination.ErrWrongOwner
	}
	key, err := json.Marshal([]string{authority.CoreID, authority.InitiatorDeviceID, authority.RequestID})
	if err != nil {
		return "", err
	}
	return "tr-owned-" + hashBytes(key), nil
}

func (s *TaskRuntimeService) ExistingOwnedDeviceTaskRequest(ctx context.Context, definitionID string, input json.RawMessage, reference *DeviceTaskDefinitionReference) (*EnqueueTaskResult, error) {
	authority, owned := coordination.FromContext(ctx)
	if !owned || definitionID == "" || len(definitionID) > 256 || !json.Valid(input) || len(input) > 512<<10 {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	id, err := OwnedRequestTaskRunID(authority)
	if err != nil {
		return nil, err
	}
	current, err := s.store.GetTaskRun(ctx, id)
	if IsTaskErrorCode(err, ErrTaskNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if current == nil || current.TaskDefinitionID != definitionID {
		return nil, coordination.ErrRequestConflict
	}
	definition, err := s.store.GetTaskDefinition(ctx, definitionID)
	if err != nil {
		return nil, err
	}
	if definition == nil {
		return nil, coordination.ErrRequestConflict
	}
	if source := definition.RemoteSource; source != nil {
		if reference == nil || source.Reference != *reference {
			return nil, coordination.ErrRequestConflict
		}
		if err := validateDeviceTaskSource(authority, definition); err != nil {
			return nil, err
		}
	} else if reference != nil {
		return nil, coordination.ErrRequestConflict
	}
	fingerprint, err := taskDefinitionFingerprint(definition)
	if err != nil {
		return nil, err
	}
	expected := CloneTaskRun(current)
	expected.TaskDefinitionID, expected.DefinitionFingerprint, expected.InputHash = definitionID, fingerprint, hashBytes(input)
	expected.ExtensionID, expected.ModuleID = definition.ExtensionID, definition.ModuleID
	expected.InvocationID, expected.ScopeSnapshotID, expected.Priority = id, id, 0
	expected.ExecutionPlacement = TaskExecutionPlacementDevice
	expected.ExecutionTarget.SpaceID, expected.ExecutionTarget.DeviceID = runtimeidentity.SpaceID(authority.CoreID), runtimeidentity.DeviceID(authority.TargetDeviceID)
	expected.ExecutionTarget.SourceTaskDefinitionID = ""
	if definition.RemoteSource != nil {
		expected.ExecutionTarget.SourceTaskDefinitionID = SourceTaskDefinitionID(definition)
	}
	return s.existingOwnedEnqueue(ctx, expected)
}

func (s *TaskRuntimeService) existingOwnedEnqueue(ctx context.Context, expected *TaskRun) (*EnqueueTaskResult, error) {
	current, err := s.store.GetTaskRun(ctx, expected.TaskRunID)
	if IsTaskErrorCode(err, ErrTaskNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	actual, inherited := coordination.FromContext(ctx)
	if current == nil || !inherited || current.TaskDefinitionID != expected.TaskDefinitionID || current.DefinitionFingerprint != expected.DefinitionFingerprint || current.InputHash != expected.InputHash || current.InvocationID != expected.InvocationID || current.ScopeSnapshotID != expected.ScopeSnapshotID || current.ExtensionID != expected.ExtensionID || current.ModuleID != expected.ModuleID || current.Priority != expected.Priority || current.EffectiveExecutionPlacement() != expected.EffectiveExecutionPlacement() || current.ExecutionTarget.SourceTaskDefinitionID != expected.ExecutionTarget.SourceTaskDefinitionID || current.ExecutionTarget.DeviceID != expected.ExecutionTarget.DeviceID || current.ExecutionTarget.SpaceID != expected.ExecutionTarget.SpaceID || current.ExecutionTarget.ProviderID != expected.ExecutionTarget.ProviderID {
		return nil, coordination.ErrRequestConflict
	}
	saved, owned, err := s.taskAuthority(ctx, current.ScopeSnapshotID, current.InvocationID, current.ExtensionID, current.ModuleID)
	if err != nil {
		return nil, err
	}
	if !owned || actual != saved || s.config.OwnedInputs == nil {
		return nil, coordination.ErrRequestConflict
	}
	if _, err := s.config.OwnedInputs.Input(ctx, current); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return &EnqueueTaskResult{TaskRunID: current.TaskRunID, Status: current.Status, Queued: current.Status == RunStatusQueued}, nil
}
