package main

import (
	"context"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func bindCoreOwnedTaskRuntime(binding *task_runtime.OwnedRuntimeBinding, runtime *devicemesh.Runtime, coreID string) error {
	if runtime == nil || runtime.Coordination == nil || coreID == "" {
		return coordination.ErrScopeExpired
	}
	return binding.Bind(func(ctx context.Context, authority coordination.ExecutionScope, _ *task_runtime.TaskRun) (context.Context, func(), error) {
		if authority.CoreID != coreID || authority.SpaceID != coreID {
			return ctx, nil, coordination.ErrScopeExpired
		}
		return runtime.Coordination.Restore(ctx, authority, runtime)
	}, runtime)
}
