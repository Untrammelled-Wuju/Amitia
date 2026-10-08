package task_runtime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type TargetTaskDefinitionPin = protocol.TargetTaskDefinitionPin

type TargetTaskDefinitionProvider interface {
	TargetTaskDefinition(context.Context, coordination.ExecutionScope, string) (TargetTaskDefinitionPin, error)
}

type OwnedTaskTargetDefinitionPort interface {
	Prepare(context.Context, *TaskRun, *TaskDefinition) (TargetTaskDefinitionPin, error)
}

type AcknowledgedTaskTargetDefinitionPort struct {
	Data     coordination.DataPort
	Provider TargetTaskDefinitionProvider
}

type ownedTargetTaskDefinition struct {
	Scope                 coordination.ExecutionScope `json:"executionScope"`
	TaskRunID             string                      `json:"taskRunId"`
	DefinitionFingerprint string                      `json:"definitionFingerprint"`
	Target                TargetTaskDefinitionPin     `json:"target"`
}

func portableTaskDefinitionFingerprint(definition *TaskDefinition) (string, error) {
	if definition == nil {
		return "", NewTaskError(ErrTaskDefinitionInvalid, "任务定义不存在")
	}
	copy, err := sourcePortableTaskDefinition(definition)
	if err != nil {
		return "", err
	}
	return taskDefinitionFingerprint(copy)
}

func ValidateTargetTaskDefinition(deviceID string, definition *TaskDefinition, target TargetTaskDefinitionPin) error {
	portable, err := portableTaskDefinitionFingerprint(definition)
	if err != nil {
		return err
	}
	if target.DeviceID != deviceID || target.TaskID != SourceTaskDefinitionID(definition) || target.ExtensionID != definition.ExtensionID || target.ModuleID != definition.ModuleID || target.InstalledGeneration < 1 || target.PortableFingerprint != portable || !validTaskFingerprint(target.DefinitionFingerprint) || target.EntryHash != definition.EntryHash || definition.RemoteSource != nil && definition.RemoteSource.Reference.DeviceID != deviceID {
		return NewTaskError(ErrTaskDefinitionInvalid, "目标设备任务与已授权插件定义不一致")
	}
	return nil
}

func validTaskFingerprint(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func (s *TaskRuntimeService) DescribeInstalledTask(ctx context.Context, taskID, deviceID string) (TargetTaskDefinitionPin, error) {
	if err := validateSourceTaskExecutionAvailable(ctx); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if len(taskID) == 0 || len(taskID) > 256 || deviceID == "" || s.config.InstalledDefinitionValidator == nil || s.config.EntryResolver == nil {
		return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskDefinitionInvalid, "目标设备的已安装任务校验端口不完整")
	}
	definition, err := s.store.GetTaskDefinition(ctx, taskID)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if definition == nil || definition.InstalledGeneration < 1 || !validTaskFingerprint(strings.TrimPrefix(definition.EntryHash, "sha256:")) {
		return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskDefinitionInvalid, "目标任务缺少安装版本或入口指纹")
	}
	if err := validateSourceTaskDeclaredCapabilities(definition); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if err := s.config.InstalledDefinitionValidator(ctx, definition); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if s.config.InstalledExecutionLease != nil {
		root, release, err := s.config.InstalledExecutionLease(ctx, definition)
		if err != nil {
			if release != nil {
				release()
			}
			return TargetTaskDefinitionPin{}, err
		}
		if release == nil || root == "" {
			if release != nil {
				release()
			}
			return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskDefinitionInvalid, "目标任务安装读取租约未确认")
		}
		defer release()
		if _, err := ResolveTaskEntry(ctx, root, definition); err != nil {
			return TargetTaskDefinitionPin{}, err
		}
	} else {
		if _, err := s.config.EntryResolver(ctx, definition); err != nil {
			return TargetTaskDefinitionPin{}, err
		}
	}
	if err := s.config.InstalledDefinitionValidator(ctx, definition); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	full, err := taskDefinitionFingerprint(definition)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	current, err := s.store.GetTaskDefinition(ctx, taskID)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	currentFingerprint, err := taskDefinitionFingerprint(current)
	if err != nil || currentFingerprint != full {
		return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskDefinitionInvalid, "任务定义在查询期间已变化")
	}
	portable, err := portableTaskDefinitionFingerprint(definition)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	return TargetTaskDefinitionPin{DeviceID: deviceID, TaskID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InstalledGeneration: definition.InstalledGeneration, DefinitionFingerprint: full, PortableFingerprint: portable, EntryHash: definition.EntryHash}, nil
}

func (p AcknowledgedTaskTargetDefinitionPort) Prepare(ctx context.Context, run *TaskRun, definition *TaskDefinition) (TargetTaskDefinitionPin, error) {
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	resources, ok := p.Data.(coordination.ResourcePort)
	if !ok || p.Provider == nil {
		return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskScopeDenied, "目标任务版本端口未就绪")
	}
	if err := validateTaskDefinition(true, run, definition); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if err := validateDeviceTaskSource(scope, definition); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if _, err := (AcknowledgedTaskInputPort{Data: p.Data}).Input(ctx, run); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	target, err := p.Provider.TargetTaskDefinition(ctx, scope, SourceTaskDefinitionID(definition))
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if err := ValidateTargetTaskDefinition(scope.TargetDeviceID, definition, target); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	id := "task/target-definition/" + run.TaskRunID
	document := ownedTargetTaskDefinition{Scope: scope, TaskRunID: run.TaskRunID, DefinitionFingerprint: run.DefinitionFingerprint, Target: target}
	body, err := json.Marshal(document)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	stored, err := resources.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	if stored != nil {
		var previous ownedTargetTaskDefinition
		if stored.Deleted || stored.Revision != 1 || stored.RoleID != scope.RoleID || stored.OwnerID != scope.ResourceOwnerID || stored.Kind != "checkpoint" || stored.ID != id || json.Unmarshal(stored.Body, &previous) != nil || previous != document {
			return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskDefinitionInvalid, "目标任务安装版本已变化，禁止自动重派旧任务")
		}
	} else {
		commitScope := scope
		commitScope.RequestID += "|task-target-definition|" + run.TaskRunID
		ack, err := p.Data.Commit(ctx, coordination.Commit{Scope: commitScope, Dependencies: []coordination.ResourceVersion{{Kind: "checkpoint", ID: "task/input/" + run.TaskRunID, Revision: 1}}, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: body}}})
		if err != nil {
			return TargetTaskDefinitionPin{}, err
		}
		if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != commitScope.RequestID || ack.Versions["checkpoint/"+id] != 1 {
			return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskScopeDenied, "目标任务版本尚未取得所有者确认")
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	return target, nil
}
