package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeRuntimeHealthAccessor struct {
	states  map[domain.RuntimeInstanceID]domain.RuntimeState
	healths map[domain.RuntimeInstanceID]domain.HealthState
}

func newFakeRuntimeHealthAccessor() *fakeRuntimeHealthAccessor {
	return &fakeRuntimeHealthAccessor{
		states:  make(map[domain.RuntimeInstanceID]domain.RuntimeState),
		healths: make(map[domain.RuntimeInstanceID]domain.HealthState),
	}
}

func (f *fakeRuntimeHealthAccessor) UpdateRuntimeHealth(runtimeID domain.RuntimeInstanceID, health domain.HealthStatus, reason string, now time.Time) error {
	f.healths[runtimeID] = domain.HealthState{Status: health, Message: reason, UpdatedAt: now}
	return nil
}

func (f *fakeRuntimeHealthAccessor) GetRuntimeState(runtimeID domain.RuntimeInstanceID) (domain.RuntimeState, error) {
	state, ok := f.states[runtimeID]
	if !ok {
		return "", errors.New("not found")
	}
	return state, nil
}

type fakeTopologyStoreForHealth struct {
	topology RuntimeTopologySnapshot
}

func (s *fakeTopologyStoreForHealth) GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (RuntimeTopologySnapshot, error) {
	return s.topology, nil
}

func (s *fakeTopologyStoreForHealth) GetDependencyGraphSnapshot(runtimeID domain.RuntimeInstanceID) (DependencyGraphSnapshot, error) {
	return DependencyGraphSnapshot{RuntimeID: runtimeID}, nil
}

func (s *fakeTopologyStoreForHealth) GetTopology(runtimeID domain.RuntimeInstanceID) (TopologyAccessor, error) {
	return nil, nil
}

func TestHealthAdapter_NewNilArgs(t *testing.T) {
	_, err := NewHealthAdapter(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}

	topo := &fakeTopologyStoreForHealth{}
	_, err = NewHealthAdapter(topo, nil, nil)
	if err == nil {
		t.Error("expected error for nil runtime accessor")
	}
}

func TestHealthAdapter_HandleHealthEvent_MapsToService(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			PluginID:  "plugin-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "bridge", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateRunning

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	err = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Generation: 1,
		Health:     domain.HealthHealthy,
		Reason:     "health_check_ok",
		Occurred:   time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, ok := adapter.GetServiceHealth("rt-1", "bridge")
	if !ok {
		t.Fatal("expected to find service health")
	}
	if snap.Health != domain.HealthHealthy {
		t.Errorf("expected healthy, got %s", snap.Health)
	}
	if snap.PluginID != "plugin-1" {
		t.Errorf("expected plugin-1, got %s", snap.PluginID)
	}
}

func TestHealthAdapter_IdempotentHealthEvent(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "bridge", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateRunning

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	err = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Health: domain.HealthHealthy, Occurred: now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Health: domain.HealthHealthy, Occurred: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, ok := adapter.GetServiceHealth("rt-1", "bridge")
	if !ok {
		t.Fatal("expected to find service health")
	}
	if !snap.LastChangedAt.Equal(now) {
		t.Errorf("expected LastChangedAt unchanged for idempotent event, got %v vs %v", snap.LastChangedAt, now)
	}
}

func TestHealthAdapter_HealthChangeUpdatesRuntime(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "bridge", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateRunning

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	err = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Health: domain.HealthUnhealthy, Reason: "process_exited",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rt.healths["rt-1"].Status != domain.HealthUnhealthy {
		t.Errorf("expected runtime health unhealthy, got %s", rt.healths["rt-1"].Status)
	}
}

func TestHealthAdapter_TerminalRuntimeNoUpdate(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "bridge", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateFailed

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	err = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Health: domain.HealthHealthy,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := rt.healths["rt-1"]; ok {
		t.Error("expected no runtime health update for terminal runtime state")
	}
}

func TestHealthAdapter_Reconcile(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "bridge", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateRunning

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	adapter.ReconcileServiceHealth("rt-1", "bridge", domain.HealthHealthy, "manual_check", time.Now())

	snap, ok := adapter.GetServiceHealth("rt-1", "bridge")
	if !ok {
		t.Fatal("expected to find service health after reconcile")
	}
	if snap.Health != domain.HealthHealthy {
		t.Errorf("expected healthy, got %s", snap.Health)
	}
}

func TestHealthAdapter_ListServiceHealth_Sorted(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "z-service", Required: true},
				{ServiceID: "a-service", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateRunning

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_ = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "z-service", Health: domain.HealthHealthy,
	})
	_ = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "a-service", Health: domain.HealthHealthy,
	})

	list := adapter.ListServiceHealth("rt-1")
	if len(list) != 2 {
		t.Fatalf("expected 2 services, got %d", len(list))
	}
	if list[0].ServiceID != "a-service" || list[1].ServiceID != "z-service" {
		t.Errorf("expected sorted by ServiceID, got [%s, %s]", list[0].ServiceID, list[1].ServiceID)
	}
}

func TestHealthAdapter_ReasonTruncated(t *testing.T) {
	topo := &fakeTopologyStoreForHealth{
		topology: RuntimeTopologySnapshot{
			RuntimeID: "rt-1",
			Services: []ServiceInstanceSnapshot{
				{ServiceID: "bridge", Required: true},
			},
		},
	}
	rt := newFakeRuntimeHealthAccessor()
	rt.states["rt-1"] = domain.RuntimeStateRunning

	adapter, err := NewHealthAdapter(topo, rt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	longReason := make([]byte, 500)
	for i := range longReason {
		longReason[i] = 'x'
	}

	ctx := context.Background()
	err = adapter.HandleSupervisorHealth(ctx, SupervisorHealthEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Health: domain.HealthHealthy, Reason: string(longReason),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap, _ := adapter.GetServiceHealth("rt-1", "bridge")
	if len(snap.Reason) > 256 {
		t.Errorf("expected reason to be truncated to 256 chars, got %d", len(snap.Reason))
	}
}

func TestServiceHealthSnapshot_Clone(t *testing.T) {
	original := ServiceHealthSnapshot{
		PluginID:      "plugin-1",
		RuntimeID:     "rt-1",
		ServiceID:     "bridge",
		Health:        domain.HealthHealthy,
		LastChangedAt: time.Now(),
		Reason:        "ok",
	}

	clone := original.Clone()
	clone.Health = domain.HealthUnhealthy
	clone.Reason = "changed"

	if original.Health != domain.HealthHealthy {
		t.Error("clone mutation should not affect original")
	}
	if original.Reason != "ok" {
		t.Error("clone Reason mutation should not affect original")
	}
}
