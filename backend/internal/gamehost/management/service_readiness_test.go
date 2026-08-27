package management

import (
	"context"
	"testing"
	"time"

	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/readiness"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type managementHandshake struct {
	states    map[string]handshake.HandshakeState
	snapshots map[string]*handshake.HandshakeSnapshot
}

func (h *managementHandshake) GetState(connectionID string) (handshake.HandshakeState, bool) {
	state, ok := h.states[connectionID]
	return state, ok
}

func (h *managementHandshake) GetSnapshot(connectionID string) *handshake.HandshakeSnapshot {
	if snapshot := h.snapshots[connectionID]; snapshot != nil {
		return snapshot.Clone()
	}
	return nil
}

func (h *managementHandshake) IsReady(connectionID string) bool {
	return h.states[connectionID] == handshake.HandshakeStateReady
}

type managementFixture struct {
	ctx         context.Context
	plugin      ghdomain.PluginDescriptor
	runtime     *ghruntime.RuntimeInstanceRef
	runtimes    *ghruntime.Manager
	topology    *ghruntime.TopologyStore
	connections *ipc.ConnectionRegistry
	handshake   *managementHandshake
	readiness   *readiness.Resolver
	service     *GameCenterManagementService
	generation  int64
}

func newManagementFixture(t *testing.T) *managementFixture {
	t.Helper()
	ctx := context.Background()
	plugin := ghdomain.PluginDescriptor{
		ID:              "management-plugin",
		ExtensionID:     "com.example.management",
		Name:            "Management plugin",
		Version:         "1.0.0",
		ProtocolVersion: protocol.ProtocolVersion,
		Services: []ghdomain.ServiceDescriptor{
			{ID: "bridge", Name: "Bridge", Kind: ghdomain.ServiceKindProcess, Required: true},
			{ID: "agent", Name: "Agent", Kind: ghdomain.ServiceKindProcess, Required: true},
			{ID: "telemetry", Name: "Telemetry", Kind: ghdomain.ServiceKindProcess, Required: false},
		},
	}
	plugins := registry.NewRegistry()
	if err := plugins.Register(ctx, plugin); err != nil {
		t.Fatal(err)
	}

	runtimes := ghruntime.NewManager(ghruntime.ManagerOptions{})
	runtimeInstance, _, err := runtimes.EnsurePrimaryRuntime(ctx, plugin.ID)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := runtimes.AllocateGeneration(runtimeInstance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimes.UpdateRuntimeState(runtimeInstance.ID, ghdomain.RuntimeStateStarting, "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := runtimes.UpdateRuntimeState(runtimeInstance.ID, ghdomain.RuntimeStateRunning, "test", time.Now()); err != nil {
		t.Fatal(err)
	}

	topology := ghruntime.NewTopologyStore()
	if err := topology.PutRuntimeGraph(runtimeInstance, plugin, map[ghdomain.ServiceID]string{
		"bridge":    "definition-bridge",
		"agent":     "definition-agent",
		"telemetry": "definition-telemetry",
	}); err != nil {
		t.Fatal(err)
	}
	for serviceID, moduleID := range map[ghdomain.ServiceID]string{
		"bridge":    "module-bridge",
		"agent":     "module-agent",
		"telemetry": "module-telemetry",
	} {
		if err := topology.BindModuleID(runtimeInstance.ID, serviceID, moduleID); err != nil {
			t.Fatal(err)
		}
	}
	accessor, err := topology.GetTopology(runtimeInstance.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, serviceDescriptor := range plugin.Services {
		if err := accessor.UpdateServiceState(serviceDescriptor.ID, ghruntime.ServiceStateStarting, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := accessor.UpdateServiceState(serviceDescriptor.ID, ghruntime.ServiceStateRunning, time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	connections := ipc.NewConnectionRegistry()
	handshakeReader := &managementHandshake{
		states:    make(map[string]handshake.HandshakeState),
		snapshots: make(map[string]*handshake.HandshakeSnapshot),
	}
	readinessResolver, err := readiness.NewResolver(runtimes, topology, connections, handshakeReader)
	if err != nil {
		t.Fatal(err)
	}

	runtimeRef, err := runtimes.GetRuntime(runtimeInstance.ID)
	if err != nil {
		t.Fatal(err)
	}
	service := NewGameCenterManagementService(GameCenterManagementServiceOptions{
		Registry:  NewGameHostPluginRegistry(plugins),
		Runtimes:  NewGameHostRuntimeManager(runtimes),
		Topology:  NewGameHostTopologyStore(topology),
		Handshake: handshakeReader,
		Readiness: readinessResolver,
	})
	return &managementFixture{
		ctx:         ctx,
		plugin:      plugin,
		runtime:     runtimeRef,
		runtimes:    runtimes,
		topology:    topology,
		connections: connections,
		handshake:   handshakeReader,
		readiness:   readinessResolver,
		service:     service,
		generation:  generation,
	}
}

func (f *managementFixture) attach(t *testing.T, serviceID ghdomain.ServiceID, generation int64, ready bool) {
	t.Helper()
	connectionID := ipc.ConnectionID("management-" + string(serviceID))
	conn := ipc.NewConnection(connectionID, ipc.Peer{
		PluginID:   f.plugin.ID,
		RuntimeID:  f.runtime.ID,
		ServiceID:  serviceID,
		Generation: generation,
	}, nil, time.Now(), nil)
	if err := f.connections.Register(conn); err != nil {
		t.Fatal(err)
	}
	state := handshake.HandshakeStateHandshaking
	if ready {
		state = handshake.HandshakeStateReady
	}
	f.handshake.states[string(connectionID)] = state
	f.handshake.snapshots[string(connectionID)] = handshake.NewHandshakeSnapshot(protocol.ProtocolVersion, nil, nil, nil, "test-sdk", "1.0.0")
}

func TestRuntimeSummaryUsesTopologyWideReadiness(t *testing.T) {
	f := newManagementFixture(t)
	f.attach(t, "bridge", f.generation, true)

	summary := f.service.aggregateRuntimeSummary(f.ctx, f.runtime, f.plugin)
	if !summary.Connected {
		t.Fatal("runtime should report a valid current-generation connection")
	}
	if summary.Ready {
		t.Fatal("runtime summary must not be ready while required service agent is disconnected")
	}

	f.attach(t, "agent", f.generation, true)
	summary = f.service.aggregateRuntimeSummary(f.ctx, f.runtime, f.plugin)
	if !summary.Ready {
		t.Fatalf("runtime summary should become ready after all required services are ready: %+v", summary)
	}
}

func TestRuntimeHandshakeStatusUsesRuntimeReadinessNotRuntimeIDAsConnectionID(t *testing.T) {
	f := newManagementFixture(t)
	f.attach(t, "bridge", f.generation, true)
	f.attach(t, "agent", f.generation, true)

	status, err := f.service.GetRuntimeHandshakeStatus(f.ctx, string(f.runtime.ID))
	if err != nil {
		t.Fatal(err)
	}
	if !status.Ready || status.HandshakeState != string(handshake.HandshakeStateReady) {
		t.Fatalf("unexpected runtime handshake status: %+v", status)
	}
	if status.Protocol != protocol.ProtocolVersion || status.SDKName != "test-sdk" {
		t.Fatalf("runtime handshake metadata was not projected from a current connection: %+v", status)
	}

	detail := f.service.aggregateRuntimeDetail(f.ctx, f.runtime, f.plugin)
	if detail.Handshake == nil || !detail.Handshake.Ready {
		t.Fatalf("runtime detail must use runtime-level handshake status: %+v", detail.Handshake)
	}
}

func TestServiceDTOUsesLogicalIDsAndCurrentGenerationReadiness(t *testing.T) {
	f := newManagementFixture(t)
	f.attach(t, "bridge", f.generation-1, true)
	f.attach(t, "agent", f.generation, true)

	list, err := f.service.ListServices(f.ctx, string(f.runtime.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 3 {
		t.Fatalf("expected 3 service DTOs, got %d", len(list.Items))
	}
	byID := make(map[string]GameServiceDTO, len(list.Items))
	for _, item := range list.Items {
		byID[item.ServiceID] = item
	}
	bridge, ok := byID["bridge"]
	if !ok {
		t.Fatalf("serviceId must be the logical service id, got %+v", list.Items)
	}
	if bridge.DefinitionID != "definition-bridge" || bridge.ModuleID != "module-bridge" {
		t.Fatalf("service bindings are incorrect: %+v", bridge)
	}
	if bridge.Connected || bridge.Ready {
		t.Fatalf("stale-generation connection must not be exposed as connected/ready: %+v", bridge)
	}
	agent := byID["agent"]
	if !agent.Connected || !agent.Ready {
		t.Fatalf("current-generation ready service was not projected correctly: %+v", agent)
	}
}

type managementHealth struct {
	services []ghruntime.ServiceHealthSnapshot
}

func (h managementHealth) GetServiceHealth(runtimeID string, serviceID string) (ghruntime.ServiceHealthSnapshot, bool) {
	for _, service := range h.services {
		if string(service.RuntimeID) == runtimeID && string(service.ServiceID) == serviceID {
			return service, true
		}
	}
	return ghruntime.ServiceHealthSnapshot{}, false
}

func (h managementHealth) ListServiceHealth(runtimeID string) []ghruntime.ServiceHealthSnapshot {
	result := make([]ghruntime.ServiceHealthSnapshot, 0, len(h.services))
	for _, service := range h.services {
		if string(service.RuntimeID) == runtimeID {
			result = append(result, service)
		}
	}
	return result
}

func TestRuntimeHandshakeStatusDoesNotReportReadyWhenLifecycleBlocksRuntime(t *testing.T) {
	f := newManagementFixture(t)
	f.attach(t, "bridge", f.generation, true)
	f.attach(t, "agent", f.generation, true)
	accessor, err := f.topology.GetTopology(f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := accessor.UpdateServiceState("agent", ghruntime.ServiceStateFailed, time.Now()); err != nil {
		t.Fatal(err)
	}

	status, err := f.service.GetRuntimeHandshakeStatus(f.ctx, string(f.runtime.ID))
	if err != nil {
		t.Fatal(err)
	}
	if status.Ready || status.HandshakeState != "blocked" {
		t.Fatalf("ready connection must not hide a required-service lifecycle blocker: %+v", status)
	}
}

func TestManagementRuntimeHealthKeepsOptionalFailureDegraded(t *testing.T) {
	f := newManagementFixture(t)
	now := time.Now()
	f.service.health = managementHealth{services: []ghruntime.ServiceHealthSnapshot{
		{PluginID: f.plugin.ID, RuntimeID: f.runtime.ID, ServiceID: "bridge", Health: ghdomain.HealthHealthy, LastChangedAt: now},
		{PluginID: f.plugin.ID, RuntimeID: f.runtime.ID, ServiceID: "agent", Health: ghdomain.HealthHealthy, LastChangedAt: now},
		{PluginID: f.plugin.ID, RuntimeID: f.runtime.ID, ServiceID: "telemetry", Health: ghdomain.HealthUnhealthy, LastChangedAt: now},
	}}

	health, err := f.service.GetRuntimeHealth(f.ctx, string(f.runtime.ID))
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != string(ghdomain.HealthDegraded) || health.Message != "optional_service_impaired" {
		t.Fatalf("optional service failure must degrade, not fail, the runtime: %+v", health)
	}
	if got := f.service.runtimeHealth(string(f.runtime.ID)); got != string(ghdomain.HealthDegraded) {
		t.Fatalf("runtime summary health drifted from authoritative aggregator: %s", got)
	}
}
