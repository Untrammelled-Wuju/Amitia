package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type EmergencyRuntimeAdapter struct {
	executor runtime.RuntimeExecutor
	manager  runtime.RuntimeManager
}

func NewEmergencyRuntimeAdapter(executor runtime.RuntimeExecutor, manager runtime.RuntimeManager) *EmergencyRuntimeAdapter {
	return &EmergencyRuntimeAdapter{
		executor: executor,
		manager:  manager,
	}
}

func (a *EmergencyRuntimeAdapter) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	return a.executor.StopRuntime(ctx, runtimeID)
}

func (a *EmergencyRuntimeAdapter) IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	if a.manager == nil {
		return false, nil
	}
	ref, err := a.manager.GetRuntime(runtimeID)
	if err != nil {
		return false, err
	}
	return domain.IsActiveRuntimeState(ref.State), nil
}

var _ control.RuntimeStopper = (*EmergencyRuntimeAdapter)(nil)
