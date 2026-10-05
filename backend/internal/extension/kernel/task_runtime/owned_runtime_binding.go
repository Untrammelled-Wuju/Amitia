package task_runtime

import (
	"context"
	"sync/atomic"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedRuntimeDependencies struct {
	guard     OwnedTaskExecutionGuard
	data      coordination.DataPort
	resources coordination.ResourcePort
}

type OwnedRuntimeBinding struct {
	dependencies atomic.Pointer[ownedRuntimeDependencies]
}

func (b *OwnedRuntimeBinding) Bind(guard OwnedTaskExecutionGuard, data coordination.DataPort) error {
	resources, ok := data.(coordination.ResourcePort)
	if b == nil || guard == nil || data == nil || !ok {
		return NewTaskError(ErrTaskScopeDenied, "任务授权恢复与所有者资源端口不完整")
	}
	if !b.dependencies.CompareAndSwap(nil, &ownedRuntimeDependencies{guard: guard, data: data, resources: resources}) {
		return NewTaskError(ErrTaskScopeDenied, "任务所有者运行依赖已绑定，禁止替换")
	}
	return nil
}

func (b *OwnedRuntimeBinding) Apply(config *TaskRuntimeConfig) {
	config.OwnedExecutionGuard = b.Restore
	config.OwnedInputs = AcknowledgedTaskInputPort{Data: b}
	config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: b}
	config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: b}
	config.OwnedProgress = AcknowledgedTaskProgressPort{Data: b}
	config.OwnedStorage = AcknowledgedTaskStoragePort{Data: b}
	config.OwnedArtifacts = AcknowledgedTaskArtifactPort{Data: b}
	config.OwnedTargetDefinitions = AcknowledgedTaskTargetDefinitionPort{Data: b, Provider: b}
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
