package task_runtime

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTaskCheckpointPort interface {
	SaveCheckpoint(context.Context, *TaskRun, *TaskCheckpoint) error
	Checkpoint(context.Context, *TaskRun, *TaskCheckpoint) (*TaskCheckpoint, error)
}

type ownedTaskCheckpoint struct {
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	ExecutionGeneration   int64                       `json:"executionGeneration,omitempty"`
	ExecutionAttemptID    string                      `json:"executionAttemptId,omitempty"`
	Checkpoint            TaskCheckpoint              `json:"checkpoint"`
	Payload               []byte                      `json:"payloadBytes"`
}

type AcknowledgedTaskCheckpointPort struct {
	Data coordination.DataPort
}

type checkpointPlannedEffect struct {
	EffectID    string `json:"effectId"`
	Method      string `json:"method"`
	ToolID      string `json:"toolId,omitempty"`
	InputHash   string `json:"inputHash,omitempty"`
	TimeoutMS   int64  `json:"timeoutMs,omitempty"`
	Type        string `json:"type,omitempty"`
	PayloadHash string `json:"payloadHash,omitempty"`
}

func validCheckpointEffectHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateCheckpointPayloadVersion(payload json.RawMessage, version int64) error {
	if !json.Valid(payload) {
		return NewTaskError(ErrTaskCheckpointIncompatible, "检查点载荷不是有效JSON")
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || trimmed[0] != '{' {

		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return NewTaskError(ErrTaskCheckpointIncompatible, "检查点载荷不是有效JSON对象")
	}
	raw, exists := fields["cursor"]
	if !exists {
		return nil
	}
	var cursor int64
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor != version || version < 1 {
		return NewTaskError(ErrTaskCheckpointIncompatible, "检查点载荷游标与已确认版本不一致")
	}
	return nil
}

func checkpointNextEffects(payload json.RawMessage) ([]checkpointPlannedEffect, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil || object == nil {
		return nil, nil
	}
	legacy, hasLegacy := object["nextEffectIds"]
	raw, hasPlan := object["nextEffects"]
	if hasLegacy && hasPlan {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "新旧后续效果清单不能同时使用")
	}
	if hasLegacy {
		var ids []string
		if len(legacy) == 0 || legacy[0] != '[' || json.Unmarshal(legacy, &ids) != nil || len(ids) == 0 || len(ids) > 32 {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "旧检查点效果清单格式无效")
		}
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			if !validTaskHostEffectID(id) {
				return nil, NewTaskError(ErrTaskCheckpointIncompatible, "旧检查点效果编号无效")
			}
			if _, exists := seen[id]; exists {
				return nil, NewTaskError(ErrTaskCheckpointIncompatible, "旧检查点效果编号重复")
			}
			seen[id] = struct{}{}
		}
		return nil, nil
	}
	if !hasPlan {
		return nil, nil
	}
	var elements []json.RawMessage
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &elements) != nil || len(elements) == 0 || len(elements) > 32 {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "检查点后续效果计划必须包含1-32项")
	}
	plans := make([]checkpointPlannedEffect, 0, len(elements))
	seen := make(map[string]struct{}, len(elements))
	for _, element := range elements {
		var plan checkpointPlannedEffect
		decoder := json.NewDecoder(bytes.NewReader(element))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&plan) != nil || !validTaskHostEffectID(plan.EffectID) {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "后续效果计划字段或编号无效")
		}
		if _, exists := seen[plan.EffectID]; exists {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "后续效果计划编号重复")
		}
		switch plan.Method {
		case "task.host.executeTool":
			if plan.ToolID == "" || len(plan.ToolID) > 256 || !validCheckpointEffectHash(plan.InputHash) ||
				plan.TimeoutMS < 0 || plan.TimeoutMS > 5000 || plan.Type != "" || plan.PayloadHash != "" {
				return nil, NewTaskError(ErrTaskCheckpointIncompatible, "后续工具计划缺少受约束的参数摘要")
			}
		case "task.host.emitEvent":
			if plan.Type == "" || len(plan.Type) > 256 || !validCheckpointEffectHash(plan.PayloadHash) ||
				plan.ToolID != "" || plan.InputHash != "" || plan.TimeoutMS != 0 {
				return nil, NewTaskError(ErrTaskCheckpointIncompatible, "后续事件计划缺少受约束的载荷摘要")
			}
		default:
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "后续效果计划使用了未授权的方法")
		}
		seen[plan.EffectID] = struct{}{}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (plan checkpointPlannedEffect) matches(method string, call TaskHostNativeCall) bool {
	if plan.Method != method || plan.EffectID != call.EffectID {
		return false
	}
	switch method {
	case "task.host.executeTool":
		return plan.ToolID == call.ToolID && plan.TimeoutMS == call.TimeoutMS && plan.InputHash == hashBytes(call.Input)
	case "task.host.emitEvent":
		return plan.Type == call.Type && plan.PayloadHash == hashBytes(call.Payload)
	}
	return false
}

func (p AcknowledgedTaskCheckpointPort) AuthorizesPostCheckpointEffect(ctx context.Context, run *TaskRun, def *TaskDefinition, method string, call TaskHostNativeCall) (bool, error) {
	if run == nil || def == nil || run.CheckpointID == nil || *run.CheckpointID == "" || !validTaskHostEffectID(call.EffectID) || run.Generation <= 1 || run.ExecutionAttemptID == "" ||
		!def.Checkpoint || def.Recoverability != CheckpointRecoverable || (def.Idempotency != Idempotent && !(def.Idempotency == "" && def.Idempotent)) {
		return false, nil
	}
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return false, err
	}
	port, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return false, NewTaskError(ErrTaskDependencyUnavailable, "检查点所有者读取端口未就绪")
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return false, err
	}
	resourceID := "task/checkpoint/" + *run.CheckpointID
	resource, err := port.Resource(ctx, scope, "checkpoint", resourceID)
	if err != nil {
		return false, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != "checkpoint" || resource.ID != resourceID || resource.Revision != 1 || len(resource.Body) > (2<<20)+(64<<10) {
		return false, NewTaskError(ErrTaskCheckpointIncompatible, "恢复后的新效果缺少原所有者确认的检查点")
	}
	var record ownedTaskCheckpoint
	if err := json.Unmarshal(resource.Body, &record); err != nil || record.Scope != scope || record.DefinitionFingerprint != run.DefinitionFingerprint {
		return false, NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点与原权限或任务定义不符")
	}

	origin := *run
	origin.Generation = record.ExecutionGeneration
	origin.ExecutionAttemptID = TaskExecutionAttemptID(record.ExecutionAttemptID)
	if record.ExecutionGeneration < 1 || record.ExecutionGeneration > run.Generation || record.ExecutionAttemptID == "" ||
		(record.ExecutionGeneration == run.Generation && record.ExecutionAttemptID != run.ExecutionAttemptID.String()) ||
		ownedStableCheckpointID(&origin, record.Checkpoint.Version) != *run.CheckpointID {
		return false, NewTaskError(ErrTaskCheckpointIncompatible, "检查点未绑定可信的原执行代次")
	}
	checkpoint, err := p.Checkpoint(ctx, run, &record.Checkpoint)
	if err != nil {
		return false, err
	}
	if err := validateRecoveryCheckpoint(run, def, checkpoint, true); err != nil {
		return false, err
	}
	ids, err := checkpointNextEffects(checkpoint.Payload)
	if err != nil {
		return false, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return false, err
	}
	for _, plan := range ids {
		if plan.matches(method, call) {
			return true, nil
		}
	}
	return false, nil
}

func (p AcknowledgedTaskCheckpointPort) SaveCheckpoint(ctx context.Context, run *TaskRun, cp *TaskCheckpoint) error {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return err
	}
	if p.Data == nil || cp == nil || cp.CheckpointID == "" || len(cp.CheckpointID) > 256 || cp.TaskRunID != run.TaskRunID || cp.InputHash != run.InputHash || cp.Version < 1 || len(cp.Payload) > 1<<20 || !json.Valid(cp.Payload) || hashBytes(cp.Payload) != cp.PayloadHash {
		return NewTaskError(ErrTaskCheckpointIncompatible, "所有者检查点参数或完整性无效")
	}

	if run.Generation < 1 || run.ExecutionAttemptID == "" ||
		cp.CheckpointID != ownedStableCheckpointID(run, cp.Version) {
		return NewTaskError(ErrTaskCheckpointIncompatible, "所有者检查点未绑定原执行代次与步骤版本")
	}
	if err := validateCheckpointPayloadVersion(cp.Payload, cp.Version); err != nil {
		return err
	}
	if _, err := checkpointNextEffects(cp.Payload); err != nil {
		return err
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return err
	}
	id := "task/checkpoint/" + cp.CheckpointID
	metadata := *cp
	metadata.Payload = nil

	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok {
		return NewTaskError(ErrTaskDependencyUnavailable, "检查点所有者读取端口未就绪")
	}
	previous, err := resources.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return err
	}
	if previous != nil {
		cursor, _, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
		if err != nil {
			return err
		}
		if cursor == nil || !ownerCheckpointConfirmed(cp.Version, cp.CheckpointID, cursor.CheckpointVersion, cursor.CheckpointID) {
			return NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点记录存在但未获提交游标确认，禁止幂等成功")
		}
		confirmed, err := p.Checkpoint(ctx, run, cp)
		if err != nil {
			return err
		}
		if confirmed.Version != cp.Version || confirmed.CreatedAt.IsZero() {
			return NewTaskError(ErrTaskCheckpointIncompatible, "所有者检查点版本或时间无效")
		}
		return coordination.ValidateCurrent(ctx)
	}
	committed, err := (AcknowledgedTaskOutcomePort{Data: p.Data}).InspectOutcome(ctx, run)
	if err != nil {
		return err
	}
	if committed != nil {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者已确认任务终态，禁止写入后续检查点")
	}
	previousCursor, cursorRevision, err := readTaskOwnerCommitCursor(ctx, p.Data, scope, run)
	if err != nil {
		return err
	}
	cursorMutation, err := ownerCursorMutation(scope, run, previousCursor, cursorRevision, cp, false)
	if err != nil {
		return err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	encoded, err := json.Marshal(ownedTaskCheckpoint{DefinitionFingerprint: run.DefinitionFingerprint, Scope: scope, ExecutionGeneration: run.Generation, ExecutionAttemptID: run.ExecutionAttemptID.String(), Checkpoint: metadata, Payload: cp.Payload})
	if err != nil {
		return err
	}
	commitScope := scope
	commitScope.RequestID += "|task-checkpoint|" + cp.CheckpointID
	ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: encoded}, cursorMutation}})
	if err != nil {
		return err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 || ack.Versions["checkpoint/"+taskOwnerCommitCursorID(run)] != cursorRevision+1 {
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

	if document.ExecutionGeneration != 0 || document.ExecutionAttemptID != "" {
		origin := *run
		origin.Generation = document.ExecutionGeneration
		origin.ExecutionAttemptID = TaskExecutionAttemptID(document.ExecutionAttemptID)
		if document.ExecutionGeneration < 1 || document.ExecutionGeneration > run.Generation ||
			document.ExecutionAttemptID == "" ||
			ownedStableCheckpointID(&origin, document.Checkpoint.Version) != metadata.CheckpointID {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "所有者检查点原执行身份或步骤版本已变化")
		}
	}
	cp := document.Checkpoint
	cp.Payload = append(json.RawMessage(nil), document.Payload...)
	if cp.CheckpointID != metadata.CheckpointID || cp.TaskRunID != run.TaskRunID || cp.Version != metadata.Version || cp.InputHash != run.InputHash || cp.InputHash != metadata.InputHash || cp.DefinitionHash != metadata.DefinitionHash || cp.PayloadHash != metadata.PayloadHash || len(cp.Payload) > 1<<20 || !json.Valid(cp.Payload) || hashBytes(cp.Payload) != cp.PayloadHash {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "任务检查点正文与已确认引用不一致")
	}
	if err := validateCheckpointPayloadVersion(cp.Payload, cp.Version); err != nil {
		return nil, err
	}
	if _, err := checkpointNextEffects(cp.Payload); err != nil {
		return nil, err
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

func validateRecoveryCheckpoint(run *TaskRun, def *TaskDefinition, cp *TaskCheckpoint, owned bool) error {
	if run == nil || def == nil {
		return NewTaskError(ErrTaskCheckpointIncompatible, "恢复任务或定义不存在")
	}
	if cp == nil {
		if owned || run.CheckpointID != nil {
			return NewTaskError(ErrTaskCheckpointIncompatible, "任务已有执行记录但缺少可信检查点，禁止从头执行")
		}
		return nil
	}
	if cp.CheckpointID == "" || cp.TaskRunID != run.TaskRunID || cp.Version < 1 || len(cp.Payload) > 1<<20 || cp.DefinitionHash != def.DefinitionHash || cp.InputHash != run.InputHash || len(cp.Payload) == 0 || !json.Valid(cp.Payload) || hashBytes(cp.Payload) != cp.PayloadHash {
		return NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点的身份、定义、输入或内容已变化")
	}
	if err := validateCheckpointPayloadVersion(cp.Payload, cp.Version); err != nil {
		return err
	}
	if _, err := checkpointNextEffects(cp.Payload); err != nil {
		return err
	}
	if run.CheckpointID != nil && cp.CheckpointID != *run.CheckpointID {
		return NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点不是当前任务已确认的检查点")
	}
	if owned && (run.CheckpointID == nil || *run.CheckpointID != cp.CheckpointID) {
		return NewTaskError(ErrTaskCheckpointIncompatible, "设备任务恢复缺少所有者确认的检查点引用")
	}
	return nil
}

func ownedStableCheckpointID(run *TaskRun, version int64) string {
	return "cp-owned-" + hashBytes([]byte(run.TaskRunID + "\x00" + run.ExecutionAttemptID.String() + "\x00" + strconv.FormatInt(run.Generation, 10) + "\x00" + strconv.FormatInt(version, 10)))[:32]
}
