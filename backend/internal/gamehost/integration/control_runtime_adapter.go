package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/readiness"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type ControlRuntimeAdapter struct {
	manager   *runtime.Manager
	readiness readiness.Reader
}

func NewControlRuntimeAdapter(manager *runtime.Manager, runtimeReadiness readiness.Reader) *ControlRuntimeAdapter {
	return &ControlRuntimeAdapter{
		manager:   manager,
		readiness: runtimeReadiness,
	}
}

func (a *ControlRuntimeAdapter) IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	if a == nil || a.manager == nil {
		return false, fmt.Errorf("gamehost control runtime adapter: runtime manager is unavailable")
	}
	rt, err := a.manager.Get(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	// A degraded runtime is still operational when only optional services are
	// impaired. Strict control readiness is checked separately through the
	// authoritative readiness resolver.
	return readiness.IsOperationalRuntimeState(rt.State), nil
}

func (a *ControlRuntimeAdapter) IsRuntimeStopping(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	if a == nil || a.manager == nil {
		return false, fmt.Errorf("gamehost control runtime adapter: runtime manager is unavailable")
	}
	rt, err := a.manager.Get(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	return rt.State == domain.RuntimeStateStopping, nil
}

func (a *ControlRuntimeAdapter) IsRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	if a == nil || a.readiness == nil {
		return false, fmt.Errorf("gamehost control runtime adapter: runtime readiness resolver is unavailable")
	}
	return a.readiness.IsReady(ctx, runtimeID)
}

func (a *ControlRuntimeAdapter) CurrentGeneration(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, error) {
	if a == nil || a.manager == nil {
		return 0, fmt.Errorf("gamehost control runtime adapter: runtime manager is unavailable")
	}
	gen, err := a.manager.GetCurrentGeneration(runtimeID)
	if err != nil {
		return 0, err
	}
	return uint64(gen), nil
}

var _ control.RuntimeReader = (*ControlRuntimeAdapter)(nil)
var _ control.RuntimeGenerationReader = (*ControlRuntimeAdapter)(nil)
