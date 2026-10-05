package task_runtime

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskCheckpointPort interface {
	SaveCheckpoint(context.Context, *TaskRun, *TaskCheckpoint) error
	Checkpoint(context.Context, *TaskRun, *TaskCheckpoint) (*TaskCheckpoint, error)
}

type ownedTaskCheckpoint struct {
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	Checkpoint            TaskCheckpoint              `json:"checkpoint"`
	Payload               []byte                      `json:"payloadBytes"`
}

type AcknowledgedTaskCheckpointPort struct {
	Data coordination.DataPort
}

func (p AcknowledgedTaskCheckpointPort) SaveCheckpoint(ctx context.Context, run *TaskRun, cp *TaskCheckpoint) error {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return err
	}
	if p.Data == nil || cp == nil || cp.CheckpointID == "" || len(cp.CheckpointID) > 256 || cp.TaskRunID != run.TaskRunID || cp.InputHash != run.InputHash || cp.Version < 1 || len(cp.Payload) > 1<<20 || !json.Valid(cp.Payload) || hashBytes(cp.Payload) != cp.PayloadHash {
		return NewTaskError(ErrTaskCheckpointIncompatible, "所有者检查点参数或完整性无效")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return err
	}
	id := "task/checkpoint/" + cp.CheckpointID
	metadata := *cp
	metadata.Payload = nil
	encoded, err := json.Marshal(ownedTaskCheckpoint{DefinitionFingerprint: run.DefinitionFingerprint, Scope: scope, Checkpoint: metadata, Payload: cp.Payload})
	if err != nil {
		return err
	}
	commitScope := scope
	commitScope.RequestID += "|task-checkpoint|" + cp.CheckpointID
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: encoded}}})
	if err != nil {
		return err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 {
		return errors.New("任务检查点所有者尚未确认保存")
	}
	return coordination.ValidateCurrent(ctx)
}

func (p AcknowledgedTaskCheckpointPort) Checkpoint(ctx context.Context, run *TaskRun, metadata *TaskCheckpoint) (*TaskCheckpoint, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return nil, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok || metadata == nil || metadata.CheckpointID == "" || metadata.TaskRunID != run.TaskRunID {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "任务检查点所有者端口或引用无效")
	}
	id := "task/checkpoint/" + metadata.CheckpointID
	resource, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != "checkpoint" || resource.ID != id || resource.Revision != 1 || len(resource.Body) > (2<<20)+(64<<10) {
		return nil, coordination.ErrWrongOwner
	}
	var document ownedTaskCheckpoint
	if json.Unmarshal(resource.Body, &document) != nil || document.Scope != scope || document.DefinitionFingerprint != run.DefinitionFingerprint {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务检查点原授权或定义已变化")
	}
	cp := document.Checkpoint
	cp.Payload = append(json.RawMessage(nil), document.Payload...)
	if cp.CheckpointID != metadata.CheckpointID || cp.TaskRunID != run.TaskRunID || cp.Version != metadata.Version || cp.InputHash != run.InputHash || cp.InputHash != metadata.InputHash || cp.DefinitionHash != metadata.DefinitionHash || cp.PayloadHash != metadata.PayloadHash || len(cp.Payload) > 1<<20 || !json.Valid(cp.Payload) || hashBytes(cp.Payload) != cp.PayloadHash {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "任务检查点正文与已确认引用不一致")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	cp.Payload = append(json.RawMessage(nil), cp.Payload...)
	return &cp, nil
}

func (s *TaskRuntimeService) readTaskCheckpoint(ctx context.Context, run *TaskRun) (*TaskCheckpoint, error) {
	cp, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
	if err != nil || cp == nil {
		return cp, err
	}
	if _, owned := coordination.FromContext(ctx); !owned {
		return cp, nil
	}
	if s.config.OwnedCheckpoints == nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务检查点所有者端口不可用")
	}
	return s.config.OwnedCheckpoints.Checkpoint(ctx, run, cp)
}
