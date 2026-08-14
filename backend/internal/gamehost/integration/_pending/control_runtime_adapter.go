package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type ControlRuntimeAdapter struct {
	manager *runtime.Manager
}

func NewControlRuntimeAdapter(manager *runtime.Manager) *ControlRuntimeAdapter {
	return &ControlRuntimeAdapter{
		manager: manager,
	}
}

func (a *ControlRuntimeAdapter) IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	rt, err := a.manager.Get(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	return rt.State == domain.RuntimeStateRunning, nil
}

func (a *ControlRuntimeAdapter) IsRuntimeStopping(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	rt, err := a.manager.Get(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	return rt.State == domain.RuntimeStateStopping, nil
}

func (a *ControlRuntimeAdapter) IsRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	rt, err := a.manager.Get(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	return rt.IsReady(), nil
}

func (a *ControlRuntimeAdapter) CurrentGeneration(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, error) {
	gen, err := a.manager.GetCurrentGeneration(runtimeID)
	if err != nil {
		return 0, err
	}
	return uint64(gen), nil
}

var _ control.RuntimeReader = (*ControlRuntimeAdapter)(nil)
var _ control.RuntimeGenerationReader = (*ControlRuntimeAdapter)(nil)
