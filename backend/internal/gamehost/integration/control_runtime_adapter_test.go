package integration

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/readiness"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type controlReadiness struct {
	ready bool
}

func (r controlReadiness) Resolve(_ context.Context, runtimeID domain.RuntimeInstanceID) (readiness.Snapshot, error) {
	return readiness.Snapshot{RuntimeID: runtimeID, Operational: true, Ready: r.ready}, nil
}

func (r controlReadiness) IsReady(context.Context, domain.RuntimeInstanceID) (bool, error) {
	return r.ready, nil
}

func (r controlReadiness) IsServiceReady(context.Context, domain.RuntimeInstanceID, domain.ServiceID) (bool, error) {
	return r.ready, nil
}

func TestControlRuntimeAdapterUsesAuthoritativeReadinessForDegradedRuntime(t *testing.T) {
	ctx := context.Background()
	manager := runtime.NewManager(runtime.ManagerOptions{})
	rt, _, err := manager.EnsurePrimaryRuntime(ctx, "plugin-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateRuntimeState(rt.ID, domain.RuntimeStateStarting, "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateRuntimeState(rt.ID, domain.RuntimeStateRunning, "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateRuntimeState(rt.ID, domain.RuntimeStateDegraded, "optional_service_impaired", time.Now()); err != nil {
		t.Fatal(err)
	}

	adapter := NewControlRuntimeAdapter(manager, controlReadiness{ready: true})
	active, err := adapter.IsRuntimeActive(ctx, rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("degraded runtime with optional impairment must remain operational")
	}
	ready, err := adapter.IsRuntimeReady(ctx, rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("control readiness must come from the authoritative resolver, not RuntimeState")
	}

	adapter = NewControlRuntimeAdapter(manager, controlReadiness{ready: false})
	ready, err = adapter.IsRuntimeReady(ctx, rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("runtime state degraded/running must not independently make control ready")
	}
}
