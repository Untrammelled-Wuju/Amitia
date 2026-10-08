package task_runtime

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type TaskDefinitionCreationStore interface {
	CreateTaskDefinition(context.Context, *TaskDefinition) (bool, error)
}

type DeviceTaskDefinitionSource struct {
	Reference      DeviceTaskDefinitionReference `json:"reference"`
	Placement      TaskExecutionPlacement        `json:"placement,omitempty"`
	ContributionID string                        `json:"contributionId,omitempty"`
}

func NewCoreDeviceTaskDefinition(coreID, deviceID string, entry DeviceTaskCatalogEntry) (*TaskDefinition, error) {
	if entry.Definition.RemoteSource != nil || !entry.Definition.ExecutionPlacement.Normalize().IsValid() || entry.Definition.ExecutionPlacement == TaskExecutionPlacementCloud {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录不能再次引用其他设备的任务")
	}
	reference, err := NewDeviceTaskDefinitionReference(coreID, deviceID, entry)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(entry.Definition)
	if err != nil || len(encoded) > 64<<10 {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, "设备任务定义超过上限")
	}
	var definition TaskDefinition
	if json.Unmarshal(encoded, &definition) != nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, "设备任务定义无效")
	}
	definition.RemoteSource = &DeviceTaskDefinitionSource{Reference: reference, Placement: definition.ExecutionPlacement, ContributionID: definition.ContributionID}
	definition.TaskID, definition.InstalledGeneration, definition.DefinitionHash = reference.CatalogID, 0, ""
	definition.ContributionID = reference.CatalogID
	definition.ExecutionPlacement = TaskExecutionPlacementDevice
	return &definition, nil
}

func SourceTaskDefinitionID(definition *TaskDefinition) string {
	if definition == nil {
		return ""
	}
	if definition.RemoteSource != nil {
		return definition.RemoteSource.Reference.SourceTaskID
	}
	return definition.TaskID
}

func sourcePortableTaskDefinition(definition *TaskDefinition) (*TaskDefinition, error) {
	if definition == nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, "任务定义不存在")
	}
	copy := *definition
	copy.InstalledGeneration, copy.DefinitionHash = 0, ""
	if source := definition.RemoteSource; source != nil {
		reference := source.Reference
		if reference.CoreID == "" || reference.DeviceID == "" || reference.SourceTaskID == "" || !validTaskFingerprint(reference.PortableFingerprint) || definition.TaskID != reference.CatalogID || definition.ContributionID != reference.CatalogID || definition.ExecutionPlacement != TaskExecutionPlacementDevice || definition.InstalledGeneration != 0 {
			return nil, NewTaskError(ErrTaskDefinitionInvalid, "远端设备任务定义引用无效")
		}
		copy.TaskID, copy.ExecutionPlacement, copy.RemoteSource = reference.SourceTaskID, source.Placement, nil
		copy.ContributionID = source.ContributionID
		portable, err := taskDefinitionFingerprint(&copy)
		if err != nil || portable != reference.PortableFingerprint {
			return nil, NewTaskError(ErrTaskDefinitionInvalid, "远端设备任务定义与原安装声明不一致")
		}
		encoded, _ := json.Marshal([]string{reference.CoreID, reference.DeviceID, reference.SourceTaskID, portable})
		if reference.CatalogID != "mesh-task-"+hashBytes(encoded) {
			return nil, NewTaskError(ErrTaskDefinitionInvalid, "远端设备任务目录身份不一致")
		}
	}
	return &copy, nil
}

func validateDeviceTaskSource(authority coordination.ExecutionScope, definition *TaskDefinition) error {
	if definition.RemoteSource == nil {
		return nil
	}
	if _, err := sourcePortableTaskDefinition(definition); err != nil {
		return err
	}
	reference := definition.RemoteSource.Reference
	if reference.CoreID != authority.CoreID || reference.DeviceID != authority.TargetDeviceID {
		return NewTaskError(ErrTaskScopeDenied, "设备任务不能转交其他 Core 或设备执行")
	}
	return nil
}

func validateSourceTaskRoot(authority coordination.ExecutionScope, run *TaskRun, definition *TaskDefinition, pin TargetTaskDefinitionPin) error {
	if run.ExecutionTarget.SourceTaskDefinitionID == "" {
		if run.TaskDefinitionID != definition.TaskID {
			return NewTaskError(ErrTaskDefinitionInvalid, "设备任务原定义身份不一致")
		}
		return nil
	}
	if run.ExecutionTarget.SourceTaskDefinitionID != definition.TaskID {
		return NewTaskError(ErrTaskDefinitionInvalid, "目标设备任务定义映射不一致")
	}
	aliased, err := NewCoreDeviceTaskDefinition(authority.CoreID, authority.TargetDeviceID, DeviceTaskCatalogEntry{Definition: *definition, Target: pin})
	if err != nil {
		return err
	}
	fingerprint, err := taskDefinitionFingerprint(aliased)
	if err != nil || run.TaskDefinitionID != aliased.TaskID || run.DefinitionFingerprint != fingerprint {
		return NewTaskError(ErrTaskDefinitionInvalid, "设备任务目录引用与 Core 原任务元数据不一致")
	}
	return nil
}

func (s *TaskRuntimeService) ImportDeviceTaskDefinition(ctx context.Context, entry DeviceTaskCatalogEntry) (*TaskDefinition, error) {
	authority, owned := coordination.FromContext(ctx)
	creator, ok := s.store.(TaskDefinitionCreationStore)
	if !owned || !ok {
		return nil, NewTaskError(ErrTaskScopeDenied, "设备任务目录导入缺少执行授权或原子存储")
	}
	definition, err := NewCoreDeviceTaskDefinition(authority.CoreID, authority.TargetDeviceID, entry)
	if err != nil {
		return nil, err
	}
	if err := validateDeviceTaskSource(authority, definition); err != nil {
		return nil, err
	}
	if err := coordination.CommitCurrent(ctx, func() error {
		_, err := creator.CreateTaskDefinition(ctx, definition)
		return err
	}); err != nil {
		return nil, err
	}
	stored, err := s.store.GetTaskDefinition(ctx, definition.TaskID)
	if err != nil {
		return nil, err
	}
	expected, err := taskDefinitionFingerprint(definition)
	if err != nil {
		return nil, err
	}
	actual, err := taskDefinitionFingerprint(stored)
	if err != nil || actual != expected {
		return nil, coordination.ErrRequestConflict
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return stored, nil
}
