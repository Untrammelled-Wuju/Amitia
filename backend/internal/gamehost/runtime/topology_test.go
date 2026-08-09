package runtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func newTestRuntime(id domain.RuntimeInstanceID, pluginID domain.PluginID, now time.Time) *domain.RuntimeInstance {
	rt, _ := domain.NewRuntimeInstance(id, pluginID, now)
	return rt
}

func newTestDescriptor(id domain.PluginID, services []domain.ServiceDescriptor) domain.PluginDescriptor {
	return domain.PluginDescriptor{
		ID:              id,
		ExtensionID:     "ext-" + string(id),
		Name:            "Test Plugin " + string(id),
		Version:         "1.0.0",
		ProtocolVersion: "1.0",
		Services:        services,
	}
}

func TestTopologyBuilder_SingleService(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "minecraft-plugin", now)
	descriptor := newTestDescriptor("minecraft-plugin", []domain.ServiceDescriptor{
		{
			ID:       "bridge",
			Name:     "Bridge Service",
			Kind:     domain.ServiceKindProcess,
			Required: true,
		},
	})

	builder := NewTopologyBuilder()
	topology, err := builder.Build(rt, descriptor, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topology.RuntimeID != "runtime-001" {
		t.Errorf("expected runtime-001, got %s", topology.RuntimeID)
	}
	if topology.PluginID != "minecraft-plugin" {
		t.Errorf("expected minecraft-plugin, got %s", topology.PluginID)
	}
	if topology.ServiceCount() != 1 {
		t.Errorf("expected 1 service, got %d", topology.ServiceCount())
	}

	svc, err := topology.GetService("bridge")
	if err != nil {
		t.Fatalf("unexpected error getting service: %v", err)
	}
	if svc.ID != "runtime-001/bridge" {
		t.Errorf("expected runtime-001/bridge, got %s", svc.ID)
	}
	if svc.ServiceID != "bridge" {
		t.Errorf("expected bridge, got %s", svc.ServiceID)
	}
}

func TestTopologyBuilder_MultiService(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "minecraft-plugin", now)
	descriptor := newTestDescriptor("minecraft-plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess, Required: true},
		{ID: "agent", Name: "Agent", Kind: domain.ServiceKindProcess, Required: true, DependsOn: []domain.ServiceID{"bridge"}},
		{ID: "vision", Name: "Vision", Kind: domain.ServiceKindExternal, Required: false, DependsOn: []domain.ServiceID{"bridge"}},
	})

	builder := NewTopologyBuilder()
	topology, err := builder.Build(rt, descriptor, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topology.ServiceCount() != 3 {
		t.Errorf("expected 3 services, got %d", topology.ServiceCount())
	}

	expectedIDs := []ServiceInstanceID{"runtime-001/bridge", "runtime-001/agent", "runtime-001/vision"}
	for _, expectedID := range expectedIDs {
		found := false
		for _, svc := range topology.ListServices() {
			if svc.ID == expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find service %s", expectedID)
		}
	}
}

func TestTopologyBuilder_StableServiceInstanceID(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "minecraft-plugin", now)
	descriptor := newTestDescriptor("minecraft-plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess, Required: true},
	})

	builder := NewTopologyBuilder()

	topology1, err := builder.Build(rt, descriptor, now)
	if err != nil {
		t.Fatalf("first build unexpected error: %v", err)
	}

	topology2, err := builder.Build(rt, descriptor, now)
	if err != nil {
		t.Fatalf("second build unexpected error: %v", err)
	}

	svc1, _ := topology1.GetService("bridge")
	svc2, _ := topology2.GetService("bridge")

	if svc1.ID != svc2.ID {
		t.Errorf("expected same ServiceInstanceID, got %s and %s", svc1.ID, svc2.ID)
	}
}

func TestTopologyBuilder_DifferentRuntimesDifferentIDs(t *testing.T) {
	now := time.Now()
	rtA := newTestRuntime("runtime-a", "minecraft-plugin", now)
	rtB := newTestRuntime("runtime-b", "minecraft-plugin", now)
	descriptor := newTestDescriptor("minecraft-plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess, Required: true},
	})

	builder := NewTopologyBuilder()
	topologyA, _ := builder.Build(rtA, descriptor, now)
	topologyB, _ := builder.Build(rtB, descriptor, now)

	svcA, _ := topologyA.GetService("bridge")
	svcB, _ := topologyB.GetService("bridge")

	if svcA.ID == svcB.ID {
		t.Error("different runtimes should have different service instance ids")
	}
	if svcA.ID != "runtime-a/bridge" {
		t.Errorf("expected runtime-a/bridge, got %s", svcA.ID)
	}
	if svcB.ID != "runtime-b/bridge" {
		t.Errorf("expected runtime-b/bridge, got %s", svcB.ID)
	}
}

func TestTopologyBuilder_RuntimePluginMismatch(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin-a", now)
	descriptor := newTestDescriptor("plugin-b", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	_, err := builder.Build(rt, descriptor, now)
	if err == nil {
		t.Fatal("expected error for plugin mismatch")
	}
	if !IsTopologyError(err, ErrPluginMismatch) {
		t.Errorf("expected plugin_mismatch error, got %v", err)
	}
}

func TestTopologyBuilder_DuplicateServiceID(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)

	descriptor := domain.PluginDescriptor{
		ID:              "plugin",
		ExtensionID:     "ext-plugin",
		Name:            "Test Plugin",
		Version:         "1.0.0",
		ProtocolVersion: "1.0",
		Services: []domain.ServiceDescriptor{
			{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
			{ID: "bridge", Name: "Bridge Duplicate", Kind: domain.ServiceKindProcess},
		},
	}

	builder := NewTopologyBuilder()
	_, err := builder.Build(rt, descriptor, now)
	if err == nil {
		t.Fatal("expected error for duplicate service id")
	}
	if !IsTopologyError(err, ErrDuplicateService) {
		t.Errorf("expected duplicate_service error, got %v", err)
	}
}

func TestTopologyBuilder_MissingDependency(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)

	descriptor := domain.PluginDescriptor{
		ID:              "plugin",
		ExtensionID:     "ext-plugin",
		Name:            "Test Plugin",
		Version:         "1.0.0",
		ProtocolVersion: "1.0",
		Services: []domain.ServiceDescriptor{
			{ID: "agent", Name: "Agent", Kind: domain.ServiceKindProcess, DependsOn: []domain.ServiceID{"bridge"}},
		},
	}

	builder := NewTopologyBuilder()
	_, err := builder.Build(rt, descriptor, now)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
	if !IsTopologyError(err, ErrDependencyNotFound) {
		t.Errorf("expected dependency_not_found error, got %v", err)
	}
}

func TestTopologyBuilder_ZeroService(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{})

	builder := NewTopologyBuilder()
	topology, err := builder.Build(rt, descriptor, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topology.ServiceCount() != 0 {
		t.Errorf("expected 0 services, got %d", topology.ServiceCount())
	}

	services := topology.ListServices()
	if len(services) != 0 {
		t.Errorf("expected empty list, got %d items", len(services))
	}
}

func TestTopologyBuilder_RequiredPreserved(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess, Required: true},
		{ID: "vision", Name: "Vision", Kind: domain.ServiceKindExternal, Required: false},
	})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	bridge, _ := topology.GetService("bridge")
	if !bridge.Required {
		t.Error("expected bridge to be required")
	}

	vision, _ := topology.GetService("vision")
	if vision.Required {
		t.Error("expected vision to not be required")
	}
}

func TestTopologyBuilder_DependenciesPreserved(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
		{ID: "agent", Name: "Agent", Kind: domain.ServiceKindProcess, DependsOn: []domain.ServiceID{"bridge"}},
	})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	agent, _ := topology.GetService("agent")
	if len(agent.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(agent.Dependencies))
	}
	if agent.Dependencies[0] != "bridge" {
		t.Errorf("expected dependency on bridge, got %s", agent.Dependencies[0])
	}
}

func TestTopologySnapshot_ListOrderStable(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "zebra", Name: "Zebra", Kind: domain.ServiceKindProcess},
		{ID: "alpha", Name: "Alpha", Kind: domain.ServiceKindProcess},
		{ID: "mango", Name: "Mango", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	services := topology.ListServices()
	expectedOrder := []domain.ServiceID{"alpha", "mango", "zebra"}
	for i, svc := range services {
		if svc.ServiceID != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], svc.ServiceID)
		}
	}
}

func TestTopologySnapshot_DeepCopy(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "agent", Name: "Agent", Kind: domain.ServiceKindProcess, DependsOn: []domain.ServiceID{"bridge"}},
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	snap := topology.Snapshot()

	for i := range snap.Services {
		snap.Services[i].Dependencies = append(snap.Services[i].Dependencies, "modified")
		snap.Services[i].Metadata = map[string]string{"modified": "true"}
	}

	agent, _ := topology.GetService("agent")
	if len(agent.Dependencies) != 1 {
		t.Error("modifying snapshot affected original Dependencies")
	}
	if agent.Metadata != nil && agent.Metadata["modified"] == "true" {
		t.Error("modifying snapshot affected original Metadata")
	}
}

func TestTopology_UpdateServiceState(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	later := now.Add(time.Second)
	err := topology.UpdateServiceState("bridge", ServiceStateStarting, later)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc, _ := topology.GetService("bridge")
	if svc.State != ServiceStateStarting {
		t.Errorf("expected starting, got %s", svc.State)
	}
}

func TestTopology_UpdateServiceState_NotFound(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	err := topology.UpdateServiceState("nonexistent", ServiceStateStarting, now)
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
	if !IsTopologyError(err, ErrNotFound) {
		t.Errorf("expected not_found, got %v", err)
	}
}

func TestTopology_SetServiceMetadata(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "plugin", now)
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	topology, _ := builder.Build(rt, descriptor, now)

	later := now.Add(time.Second)
	err := topology.SetServiceMetadata("bridge", "node", "node-01", later)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc, _ := topology.GetService("bridge")
	if svc.Metadata["node"] != "node-01" {
		t.Errorf("expected node=node-01, got %s", svc.Metadata["node"])
	}
}

func TestTopology_NilRuntime(t *testing.T) {
	now := time.Now()
	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	_, err := builder.Build(nil, descriptor, now)
	if err == nil {
		t.Fatal("expected error for nil runtime")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestTopology_GetServiceNotFound(t *testing.T) {
	now := time.Now()
	topology := NewRuntimeTopology("rt-001", "plugin", now)

	_, err := topology.GetService("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
	if !IsTopologyError(err, ErrNotFound) {
		t.Errorf("expected not_found, got %v", err)
	}
}

func TestTopology_AddDuplicateService(t *testing.T) {
	now := time.Now()
	topology := NewRuntimeTopology("rt-001", "plugin", now)

	svc1, _ := NewServiceInstance("rt-001/svc", "rt-001", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)
	svc2, _ := NewServiceInstance("rt-001/svc", "rt-001", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	if err := topology.AddService(svc1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := topology.AddService(svc2); err == nil {
		t.Fatal("expected error for duplicate service")
	}
}

func TestTopology_NilDescriptorID(t *testing.T) {
	now := time.Now()
	rt := newTestRuntime("runtime-001", "", now)
	descriptor := newTestDescriptor("", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	_, err := builder.Build(rt, descriptor, now)
	if err == nil {
		t.Fatal("expected error for empty descriptor ID")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestTopology_MultipleRuntimesForSamePlugin(t *testing.T) {
	now := time.Now()
	rt1 := newTestRuntime("runtime-001", "minecraft-plugin", now)
	rt2 := newTestRuntime("runtime-002", "minecraft-plugin", now)
	descriptor := newTestDescriptor("minecraft-plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
		{ID: "agent", Name: "Agent", Kind: domain.ServiceKindProcess, DependsOn: []domain.ServiceID{"bridge"}},
	})

	builder := NewTopologyBuilder()
	topology1, err := builder.Build(rt1, descriptor, now)
	if err != nil {
		t.Fatalf("first build error: %v", err)
	}
	topology2, err := builder.Build(rt2, descriptor, now)
	if err != nil {
		t.Fatalf("second build error: %v", err)
	}

	if topology1.ServiceCount() != 2 {
		t.Errorf("topology1 expected 2 services, got %d", topology1.ServiceCount())
	}
	if topology2.ServiceCount() != 2 {
		t.Errorf("topology2 expected 2 services, got %d", topology2.ServiceCount())
	}

	bridge1, _ := topology1.GetService("bridge")
	bridge2, _ := topology2.GetService("bridge")
	if bridge1.ID == bridge2.ID {
		t.Error("different runtimes should have different bridge IDs")
	}
}

func TestTopologyBuilder_InvalidState(t *testing.T) {
	now := time.Now()
	rt, _ := domain.NewRuntimeInstance("runtime-001", "plugin", now)
	rt.State = domain.RuntimeStateRunning

	descriptor := newTestDescriptor("plugin", []domain.ServiceDescriptor{
		{ID: "bridge", Name: "Bridge", Kind: domain.ServiceKindProcess},
	})

	builder := NewTopologyBuilder()
	_, err := builder.Build(rt, descriptor, now)
	if err == nil {
		t.Fatal("expected error when runtime is not in created state")
	}
	if !IsTopologyError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state, got %v", err)
	}
}
