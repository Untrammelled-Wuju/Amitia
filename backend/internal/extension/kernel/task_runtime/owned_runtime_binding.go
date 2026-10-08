package task_runtime

import (
	"context"
	"sync/atomic"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedRuntimeDependencies struct {
	guard     OwnedTaskExecutionGuard
	public    PublicTaskRequestGuard
	data      coordination.DataPort
	resources coordination.ResourcePort
}

type OwnedRuntimeBinding struct {
	dependencies atomic.Pointer[ownedRuntimeDependencies]
}

func (b *OwnedRuntimeBinding) Bind(guard OwnedTaskExecutionGuard, data coordination.DataPort, public ...PublicTaskRequestGuard) error {
	resources, ok := data.(coordination.ResourcePort)
	if b == nil || guard == nil || data == nil || !ok {
		return NewTaskError(ErrTaskScopeDenied, "任务授权恢复与所有者资源端口不完整")
	}
	dependencies := &ownedRuntimeDependencies{guard: guard, data: data, resources: resources}
	if len(public) > 1 {
		return NewTaskError(ErrTaskScopeDenied, "任务管理授权端口不能重复绑定")
	}
	if len(public) == 1 {
		dependencies.public = public[0]
	}
	if !b.dependencies.CompareAndSwap(nil, dependencies) {
		return NewTaskError(ErrTaskScopeDenied, "任务所有者运行依赖已绑定，禁止替换")
	}
	return nil
}

func (b *OwnedRuntimeBinding) Apply(config *TaskRuntimeConfig) {
	config.OwnedExecutionGuard = b.Restore
	config.OwnedReadGuard = b.OpenRead
	config.PublicRequestGuard = b.GuardPublicRequest
	config.OwnedInputs = AcknowledgedTaskInputPort{Data: b}
	config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: b}
	config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: b}
	config.OwnedProgress = AcknowledgedTaskProgressPort{Data: b}
	config.OwnedStorage = AcknowledgedTaskStoragePort{Data: b}
	config.OwnedArtifacts = AcknowledgedTaskArtifactPort{Data: b}
	config.OwnedTargetDefinitions = AcknowledgedTaskTargetDefinitionPort{Data: b, Provider: b}
	config.OwnedTargetPermissions = AcknowledgedTaskPermissionPort{Provider: b}
}

func (b *OwnedRuntimeBinding) GuardPublicRequest(ctx context.Context, authority *coordination.ExecutionScope) (context.Context, func(), error) {
	dependencies, err := b.load()
	if err != nil {
		return ctx, nil, err
	}
	if dependencies.public == nil {
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务管理授权端口尚未就绪")
	}
	return dependencies.public(ctx, authority)
}

func (b *OwnedRuntimeBinding) TargetTaskDefinition(ctx context.Context, scope coordination.ExecutionScope, id string) (TargetTaskDefinitionPin, error) {
	dependencies, err := b.load()
	if err != nil {
		return TargetTaskDefinitionPin{}, err
	}
	provider, ok := dependencies.data.(TargetTaskDefinitionProvider)
	if !ok {
		return TargetTaskDefinitionPin{}, NewTaskError(ErrTaskScopeDenied, "目标设备任务版本查询尚未就绪")
	}
	return provider.TargetTaskDefinition(ctx, scope, id)
}

func (b *OwnedRuntimeBinding) TargetTaskPermissions(ctx context.Context, request SourceTaskPermissionRequest) error {
	dependencies, err := b.load()
	if err != nil {
		return err
	}
	provider, ok := dependencies.data.(TargetTaskPermissionProvider)
	if !ok {
		return NewTaskError(ErrTaskPermissionDenied, "目标设备资源权限确认端口尚未就绪")
	}
	return provider.TargetTaskPermissions(ctx, request)
}

func (b *OwnedRuntimeBinding) load() (*ownedRuntimeDependencies, error) {
	if b != nil {
		if dependencies := b.dependencies.Load(); dependencies != nil {
			return dependencies, nil
		}
	}
	return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者运行依赖尚未就绪")
}

func (b *OwnedRuntimeBinding) Restore(ctx context.Context, expected coordination.ExecutionScope, run *TaskRun) (context.Context, func(), error) {
	dependencies, err := b.load()
	if err != nil {
		return ctx, nil, err
	}
	return dependencies.guard(ctx, expected, run)
}

func (b *OwnedRuntimeBinding) OpenRead(ctx context.Context, expected coordination.ExecutionScope, run *TaskRun) (context.Context, func(), error) {
	dependencies, err := b.load()
	if err != nil {
		return ctx, nil, err
	}
	reader, ok := dependencies.data.(coordination.TaskHistoryAuthorityPort)
	if !ok || run == nil {
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务历史只读授权端口尚未就绪")
	}
	return reader.OpenTaskRead(ctx, coordination.TaskReadProof{Scope: expected, TaskRunID: run.TaskRunID, DefinitionFingerprint: run.DefinitionFingerprint})
}

func (b *OwnedRuntimeBinding) Roles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	dependencies, err := b.load()
	if err != nil {
		return nil, err
	}
	return dependencies.data.Roles(ctx, scope)
}

func (b *OwnedRuntimeBinding) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	dependencies, err := b.load()
	if err != nil {
		return coordination.DataSnapshot{}, err
	}
	return dependencies.data.Snapshot(ctx, scope, query)
}

func (b *OwnedRuntimeBinding) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	dependencies, err := b.load()
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	return dependencies.data.Commit(ctx, commit)
}

func (b *OwnedRuntimeBinding) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	dependencies, err := b.load()
	if err != nil {
		return nil, err
	}
	return dependencies.resources.Resource(ctx, scope, kind, id)
}
