package runtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestHealthAggregator_AllHealthy(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		PluginID:  "plugin-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", State: ServiceStateRunning, Required: true},
			{ServiceID: "agent", State: ServiceStateRunning, Required: true},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "bridge", Health: domain.HealthHealthy, LastChangedAt: now},
		{ServiceID: "agent", Health: domain.HealthHealthy, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)
	if result.Health != domain.HealthHealthy {
		t.Errorf("expected healthy, got %s", result.Health)
	}
}

func TestHealthAggregator_RequiredUnhealthy(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", Required: true},
			{ServiceID: "agent", Required: true},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "bridge", Health: domain.HealthUnhealthy, LastChangedAt: now},
		{ServiceID: "agent", Health: domain.HealthHealthy, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)
	if result.Health != domain.HealthUnhealthy {
		t.Errorf("expected unhealthy, got %s", result.Health)
	}
}

func TestHealthAggregator_OptionalUnhealthy(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", Required: true},
			{ServiceID: "telemetry", Required: false},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "bridge", Health: domain.HealthHealthy, LastChangedAt: now},
		{ServiceID: "telemetry", Health: domain.HealthUnhealthy, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)
	if result.Health != domain.HealthDegraded {
		t.Errorf("expected degraded, got %s", result.Health)
	}
}

func TestHealthAggregator_RequiredDegraded(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", Required: true},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "bridge", Health: domain.HealthDegraded, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)
	if result.Health != domain.HealthDegraded {
		t.Errorf("expected degraded, got %s", result.Health)
	}
}

func TestHealthAggregator_StartingServiceUnknown(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", Required: true},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "bridge", Health: domain.HealthUnknown, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)
	if result.Health != domain.HealthDegraded {
		t.Errorf("expected degraded for unknown starting service, got %s", result.Health)
	}
}

func TestHealthAggregator_ZeroService(t *testing.T) {
	a := NewHealthAggregator()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services:  []ServiceInstanceSnapshot{},
	}

	result := a.AggregateRuntimeHealth(topology, nil)
	if result.Health != domain.HealthHealthy {
		t.Errorf("expected healthy for zero-service runtime, got %s", result.Health)
	}
}

func TestHealthAggregator_OnlyOptionalUnhealthy(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "telemetry", Required: false},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "telemetry", Health: domain.HealthUnhealthy, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)
	if result.Health != domain.HealthDegraded {
		t.Errorf("expected degraded, got %s", result.Health)
	}
}

func TestHealthAggregator_Idempotent(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", Required: true},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "bridge", Health: domain.HealthHealthy, LastChangedAt: now},
	}

	r1 := a.AggregateRuntimeHealth(topology, services)
	r2 := a.AggregateRuntimeHealth(topology, services)

	if r1.Health != r2.Health {
		t.Errorf("expected idempotent result, got %s vs %s", r1.Health, r2.Health)
	}
}

func TestHealthAggregator_ListOrderStable(t *testing.T) {
	a := NewHealthAggregator()
	now := time.Now()

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "z-service", Required: true},
			{ServiceID: "a-service", Required: true},
			{ServiceID: "m-service", Required: true},
		},
	}

	services := []ServiceHealthSnapshot{
		{ServiceID: "z-service", Health: domain.HealthHealthy, LastChangedAt: now},
		{ServiceID: "a-service", Health: domain.HealthHealthy, LastChangedAt: now},
		{ServiceID: "m-service", Health: domain.HealthHealthy, LastChangedAt: now},
	}

	result := a.AggregateRuntimeHealth(topology, services)

	expected := []domain.ServiceID{"a-service", "m-service", "z-service"}
	for i, snap := range result.ServiceHealths {
		if snap.ServiceID != expected[i] {
			t.Errorf("expected service %s at index %d, got %s", expected[i], i, snap.ServiceID)
		}
	}
}

func TestQuarantineImpactOnRuntime_Required(t *testing.T) {
	services := []ServiceInstanceSnapshot{
		{ServiceID: "bridge", Required: true},
	}

	impact := QuarantineImpactOnRuntime(nil, "bridge", services)
	if impact != QuarantineImpactRuntimeFailed {
		t.Errorf("expected runtime_failed for required quarantine, got %s", impact)
	}
}

func TestQuarantineImpactOnRuntime_Optional(t *testing.T) {
	services := []ServiceInstanceSnapshot{
		{ServiceID: "bridge", Required: true},
		{ServiceID: "telemetry", Required: false},
	}

	impact := QuarantineImpactOnRuntime(nil, "telemetry", services)
	if impact != QuarantineImpactRuntimeDegraded {
		t.Errorf("expected runtime_degraded for optional quarantine, got %s", impact)
	}
}

func TestQuarantineImpactOnRuntime_BlocksRequiredDependent(t *testing.T) {
	builder := NewDependencyGraphBuilder()
	topology := &RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "vision", Dependencies: nil},
			{ServiceID: "planner", Dependencies: []domain.ServiceID{"vision"}},
		},
	}
	graph, err := builder.Build(topology)
	if err != nil {
		t.Fatalf("failed to build graph: %v", err)
	}

	services := []ServiceInstanceSnapshot{
		{ServiceID: "vision", Required: false},
		{ServiceID: "planner", Required: true},
	}

	impact := QuarantineImpactOnRuntime(graph, "vision", services)
	if impact != QuarantineImpactRuntimeFailed {
		t.Errorf("expected runtime_failed when optional blocks required dependent, got %s", impact)
	}
}
