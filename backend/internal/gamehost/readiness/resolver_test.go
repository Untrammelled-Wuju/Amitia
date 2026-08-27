package readiness

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type fakeHandshakeReader struct {
	ready map[string]bool
}

func (f *fakeHandshakeReader) IsReady(connectionID string) bool {
	return f != nil && f.ready[connectionID]
}

type readinessFixture struct {
	ctx         context.Context
	runtimes    *runtime.Manager
	topology    *runtime.TopologyStore
	connections *ipc.ConnectionRegistry
	handshake   *fakeHandshakeReader
	resolver    *Resolver
	runtimeID   domain.RuntimeInstanceID
	pluginID    domain.PluginID
	generation  int64
}

func newReadinessFixture(t *testing.T, services []domain.ServiceDescriptor, state domain.RuntimeState) *readinessFixture {
	t.Helper()
	ctx := context.Background()
	pluginID := domain.PluginID("plugin-readiness")
	runtimes := runtime.NewManager(runtime.ManagerOptions{})
	rt, _, err := runtimes.EnsurePrimaryRuntime(ctx, pluginID)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := runtimes.AllocateGeneration(rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimes.UpdateRuntimeState(rt.ID, domain.RuntimeStateStarting, "test", time.Now()); err != nil {
		t.Fatal(err)
	}
	if state == domain.RuntimeStateRunning || state == domain.RuntimeStateDegraded {
		if err := runtimes.UpdateRuntimeState(rt.ID, domain.RuntimeStateRunning, "test", time.Now()); err != nil {
			t.Fatal(err)
		}
		if state == domain.RuntimeStateDegraded {
			if err := runtimes.UpdateRuntimeState(rt.ID, domain.RuntimeStateDegraded, "optional service degraded", time.Now()); err != nil {
				t.Fatal(err)
			}
		}
	} else if state != domain.RuntimeStateStarting {
		if err := runtimes.UpdateRuntimeState(rt.ID, domain.RuntimeStateRunning, "test", time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := runtimes.UpdateRuntimeState(rt.ID, state, "test", time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	plugin := domain.PluginDescriptor{
		ID:              pluginID,
		ExtensionID:     "extension-readiness",
		Name:            "Readiness fixture",
		Version:         "1.0.0",
		ProtocolVersion: "1.0",
		Services:        services,
	}
	topology := runtime.NewTopologyStore()
	definitionIDs := make(map[domain.ServiceID]string, len(services))
	for _, service := range services {
		definitionIDs[service.ID] = "definition-" + string(service.ID)
	}
	if err := topology.PutRuntimeGraph(rt, plugin, definitionIDs); err != nil {
		t.Fatal(err)
	}
	accessor, err := topology.GetTopology(rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range services {
		if err := accessor.UpdateServiceState(service.ID, runtime.ServiceStateStarting, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := accessor.UpdateServiceState(service.ID, runtime.ServiceStateRunning, time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	connections := ipc.NewConnectionRegistry()
	handshake := &fakeHandshakeReader{ready: make(map[string]bool)}
	resolver, err := NewResolver(runtimes, topology, connections, handshake)
	if err != nil {
		t.Fatal(err)
	}
	return &readinessFixture{
		ctx:         ctx,
		runtimes:    runtimes,
		topology:    topology,
		connections: connections,
		handshake:   handshake,
		resolver:    resolver,
		runtimeID:   rt.ID,
		pluginID:    pluginID,
		generation:  generation,
	}
}

func (f *readinessFixture) attach(t *testing.T, serviceID domain.ServiceID, generation int64, ready bool) ipc.ConnectionID {
	t.Helper()
	connectionID := ipc.ConnectionID("conn-" + string(serviceID))
	conn := ipc.NewConnection(connectionID, ipc.Peer{
		PluginID:   f.pluginID,
		RuntimeID:  f.runtimeID,
		ServiceID:  serviceID,
		Generation: generation,
	}, nil, time.Now(), nil)
	if err := f.connections.Register(conn); err != nil {
		t.Fatal(err)
	}
	f.handshake.ready[string(connectionID)] = ready
	return connectionID
}

func requiredService(id string) domain.ServiceDescriptor {
	return domain.ServiceDescriptor{ID: domain.ServiceID(id), Name: id, Kind: domain.ServiceKindProcess, Required: true}
}

func optionalService(id string) domain.ServiceDescriptor {
	return domain.ServiceDescriptor{ID: domain.ServiceID(id), Name: id, Kind: domain.ServiceKindProcess, Required: false}
}

func TestResolverRequiresEveryRequiredServiceInCurrentGeneration(t *testing.T) {
	f := newReadinessFixture(t, []domain.ServiceDescriptor{requiredService("bridge"), requiredService("agent")}, domain.RuntimeStateRunning)
	f.attach(t, "bridge", f.generation, true)

	snapshot, err := f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ready {
		t.Fatal("runtime must not be ready while a required service is disconnected")
	}
	if !snapshot.Connected {
		t.Fatal("runtime should report that at least one current-generation service is connected")
	}
	if snapshot.Reason != ReasonRequiredServiceDisconnected {
		t.Fatalf("reason=%s want=%s", snapshot.Reason, ReasonRequiredServiceDisconnected)
	}

	f.attach(t, "agent", f.generation, true)
	snapshot, err = f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Ready || snapshot.Reason != ReasonReady {
		t.Fatalf("runtime should be ready after all required services handshake: %+v", snapshot)
	}
}

func TestResolverRejectsStaleGenerationConnection(t *testing.T) {
	f := newReadinessFixture(t, []domain.ServiceDescriptor{requiredService("bridge")}, domain.RuntimeStateRunning)
	f.attach(t, "bridge", f.generation-1, true)

	snapshot, err := f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ready || snapshot.Connected {
		t.Fatalf("stale connection must not satisfy current-generation readiness: %+v", snapshot)
	}
	service, ok := snapshot.Service("bridge")
	if !ok || !service.Attached || service.Connected {
		t.Fatalf("stale connection projection is incorrect: %+v ok=%v", service, ok)
	}
	if snapshot.Reason != ReasonRequiredServiceStale {
		t.Fatalf("reason=%s want=%s", snapshot.Reason, ReasonRequiredServiceStale)
	}
}

func TestResolverRequiresHandshakeAndRunningService(t *testing.T) {
	f := newReadinessFixture(t, []domain.ServiceDescriptor{requiredService("bridge")}, domain.RuntimeStateRunning)
	f.attach(t, "bridge", f.generation, false)

	snapshot, err := f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ready || snapshot.Reason != ReasonRequiredServiceNotReady {
		t.Fatalf("pending handshake must block readiness: %+v", snapshot)
	}

	f.handshake.ready["conn-bridge"] = true
	accessor, err := f.topology.GetTopology(f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := accessor.UpdateServiceState("bridge", runtime.ServiceStateStopping, time.Now()); err != nil {
		t.Fatal(err)
	}
	snapshot, err = f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ready || snapshot.Reason != ReasonRequiredServiceNotRunning {
		t.Fatalf("non-running required service must block readiness: %+v", snapshot)
	}
}

func TestResolverOptionalServiceDoesNotBlockDegradedRuntime(t *testing.T) {
	f := newReadinessFixture(t, []domain.ServiceDescriptor{requiredService("bridge"), optionalService("telemetry")}, domain.RuntimeStateDegraded)
	f.attach(t, "bridge", f.generation, true)

	snapshot, err := f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Operational || !snapshot.Ready {
		t.Fatalf("optional service outage must not block a degraded-but-operational runtime: %+v", snapshot)
	}
	telemetry, ok := snapshot.Service("telemetry")
	if !ok || telemetry.Ready {
		t.Fatalf("optional service should remain independently not-ready: %+v ok=%v", telemetry, ok)
	}
}

func TestResolverDoesNotDeclareOptionalOnlyRuntimeReadyWithoutAService(t *testing.T) {
	f := newReadinessFixture(t, []domain.ServiceDescriptor{optionalService("telemetry")}, domain.RuntimeStateRunning)

	snapshot, err := f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ready || snapshot.Reason != ReasonNoServiceReady {
		t.Fatalf("optional-only runtime must not become vacuously ready: %+v", snapshot)
	}

	f.attach(t, "telemetry", f.generation, true)
	snapshot, err = f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Ready {
		t.Fatalf("optional-only runtime should become ready once a service is actually ready: %+v", snapshot)
	}
}

func TestResolverNonOperationalRuntimeNeverReady(t *testing.T) {
	f := newReadinessFixture(t, []domain.ServiceDescriptor{requiredService("bridge")}, domain.RuntimeStateSuspended)
	f.attach(t, "bridge", f.generation, true)

	snapshot, err := f.resolver.Resolve(f.ctx, f.runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ready || snapshot.Reason != ReasonRuntimeNotOperational {
		t.Fatalf("suspended runtime must not be ready: %+v", snapshot)
	}
}
