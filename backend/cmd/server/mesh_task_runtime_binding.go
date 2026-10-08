package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"

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
	}, runtime, func(ctx context.Context, _ *coordination.ExecutionScope) (context.Context, func(), error) {
		actor, ok := auth.FromContext(ctx)
		if !ok || actor == nil || actor.SpaceID.String() != coreID {
			return ctx, nil, coordination.ErrWrongOwner
		}
		if actor.PrincipalType == auth.PrincipalLocalUI && actor.HasPermission(auth.PermSystemAdmin) {
			return ctx, func() {}, nil
		}
		if actor.PrincipalType != auth.PrincipalTrustedDevice || actor.DeviceID == "" {
			return ctx, nil, coordination.ErrWrongOwner
		}
		request, authority, finish, err := runtime.Coordination.Begin(ctx, coreID, actor.DeviceID.String(), actor.DeviceID.String(), coreID, "", "task-management-"+uuid.NewString())
		if err != nil {
			return ctx, nil, err
		}
		if actor.HasPermission(auth.PermSystemAdmin) {
			policy, err := runtime.Coordination.Get(request, coreID, actor.DeviceID.String())
			if err != nil || !policy.Coordinated || !policy.Administrator || policy.PermissionRevision != authority.PermissionRevision {
				finish()
				return ctx, nil, coordination.ErrScopeExpired
			}
		}
		guarded, err := coordination.WithRequestAuthority(ctx, request)
		if err != nil {
			finish()
			return ctx, nil, err
		}
		return guarded, finish, nil
	})
}
