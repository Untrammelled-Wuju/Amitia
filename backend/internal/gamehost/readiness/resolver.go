// Package readiness owns the authoritative GameHost runtime readiness decision.
//
// Lifecycle state, process state and protocol readiness are intentionally
// different concepts. A runtime is ready only when it is operational and every
// required service for the current runtime generation has a live, fully
// handshaken connection. Optional services never block runtime readiness.
package readiness

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

// Reason is a stable diagnostic for why a runtime is not ready.
type Reason string

const (
	ReasonReady                       Reason = "ready"
	ReasonRuntimeNotOperational       Reason = "runtime_not_operational"
	ReasonNoActiveGeneration          Reason = "no_active_generation"
	ReasonTopologyEmpty               Reason = "topology_empty"
	ReasonRequiredServiceNotRunning   Reason = "required_service_not_running"
	ReasonRequiredServiceDisconnected Reason = "required_service_disconnected"
	ReasonRequiredServiceStale        Reason = "required_service_stale_generation"
	ReasonRequiredServiceNotReady     Reason = "required_service_handshake_not_ready"
	ReasonNoServiceReady              Reason = "no_service_ready"
)

// ServiceSnapshot is the readiness projection for one service in a runtime.
// Attached means an active registry entry exists. Connected is stricter: the
// connection must belong to the current plugin/runtime generation. Ready also
// requires the service lifecycle to be running and its handshake to be ready.
type ServiceSnapshot struct {
	ServiceID            domain.ServiceID
	Required             bool
	State                runtime.ServiceRuntimeState
	Attached             bool
	Connected            bool
	HandshakeReady       bool
	Ready                bool
	ConnectionID         ipc.ConnectionID
	ConnectionGeneration int64
}

// Snapshot is the single authoritative readiness projection consumed by the
// control plane, Agent bridge and Game Center management layer.
type Snapshot struct {
	RuntimeID  domain.RuntimeInstanceID
	PluginID   domain.PluginID
	State      domain.RuntimeState
	Generation int64

	Operational bool
	Connected   bool
	Ready       bool
	Reason      Reason

	RequiredServiceCount int
	Services             []ServiceSnapshot
}

// Service returns the readiness projection for a declared service.
func (s Snapshot) Service(serviceID domain.ServiceID) (ServiceSnapshot, bool) {
	for _, service := range s.Services {
		if service.ServiceID == serviceID {
			return service, true
		}
	}
	return ServiceSnapshot{}, false
}

// Reader is the shared runtime-readiness contract. Production code must read
// runtime readiness through this interface instead of deriving it independently
// from RuntimeState or the currently visible connection set.
type Reader interface {
	Resolve(ctx context.Context, runtimeID domain.RuntimeInstanceID) (Snapshot, error)
	IsReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
	IsServiceReady(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (bool, error)
}

type runtimeReader interface {
	Get(ctx context.Context, runtimeID domain.RuntimeInstanceID) (*domain.RuntimeInstance, error)
	GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
}

type topologyReader interface {
	GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (runtime.RuntimeTopologySnapshot, error)
}

type connectionReader interface {
	FindByPeer(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (*ipc.Connection, bool)
}

type handshakeReader interface {
	IsReady(connectionID string) bool
}

// Resolver combines host lifecycle, topology, connection generation and
// handshake state into one fail-closed readiness decision.
type Resolver struct {
	runtimes    runtimeReader
	topology    topologyReader
	connections connectionReader
	handshake   handshakeReader
}

func NewResolver(
	runtimes runtimeReader,
	topology topologyReader,
	connections connectionReader,
	handshake handshakeReader,
) (*Resolver, error) {
	if runtimes == nil || topology == nil || connections == nil || handshake == nil {
		return nil, fmt.Errorf("gamehost readiness: runtime, topology, connection and handshake readers are required")
	}
	return &Resolver{
		runtimes:    runtimes,
		topology:    topology,
		connections: connections,
		handshake:   handshake,
	}, nil
}

func IsOperationalRuntimeState(state domain.RuntimeState) bool {
	return state == domain.RuntimeStateRunning || state == domain.RuntimeStateDegraded
}

func (r *Resolver) Resolve(ctx context.Context, runtimeID domain.RuntimeInstanceID) (Snapshot, error) {
	if r == nil {
		return Snapshot{}, fmt.Errorf("gamehost readiness: resolver is nil")
	}
	if runtimeID == "" {
		return Snapshot{}, fmt.Errorf("gamehost readiness: runtime id is required")
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	rt, err := r.runtimes.Get(ctx, runtimeID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("gamehost readiness: get runtime %s: %w", runtimeID, err)
	}
	generation, err := r.runtimes.GetCurrentGeneration(runtimeID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("gamehost readiness: get runtime %s generation: %w", runtimeID, err)
	}
	topology, err := r.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("gamehost readiness: get runtime %s topology: %w", runtimeID, err)
	}
	if topology.RuntimeID != runtimeID || topology.PluginID != rt.PluginID {
		return Snapshot{}, fmt.Errorf("gamehost readiness: topology identity mismatch for runtime %s", runtimeID)
	}

	snapshot := Snapshot{
		RuntimeID:   runtimeID,
		PluginID:    rt.PluginID,
		State:       rt.State,
		Generation:  generation,
		Operational: IsOperationalRuntimeState(rt.State),
		Reason:      ReasonReady,
		Services:    make([]ServiceSnapshot, 0, len(topology.Services)),
	}

	allRequiredReady := true
	anyServiceReady := false
	var firstRequiredFailure Reason

	for _, service := range topology.Services {
		serviceSnapshot := ServiceSnapshot{
			ServiceID: service.ServiceID,
			Required:  service.Required,
			State:     service.State,
		}
		if service.Required {
			snapshot.RequiredServiceCount++
		}

		conn, found := r.connections.FindByPeer(runtimeID, service.ServiceID)
		if found && conn != nil && conn.IsActive() {
			serviceSnapshot.Attached = true
			serviceSnapshot.ConnectionID = conn.ID
			serviceSnapshot.ConnectionGeneration = conn.Peer.Generation

			identityMatches := conn.Peer.RuntimeID == runtimeID && conn.Peer.PluginID == rt.PluginID && conn.Peer.ServiceID == service.ServiceID
			generationMatches := generation > 0 && conn.Peer.Generation == generation
			serviceSnapshot.Connected = identityMatches && generationMatches
			if serviceSnapshot.Connected {
				snapshot.Connected = true
				serviceSnapshot.HandshakeReady = r.handshake.IsReady(string(conn.ID))
			}
		}

		serviceSnapshot.Ready = service.State == runtime.ServiceStateRunning && serviceSnapshot.Connected && serviceSnapshot.HandshakeReady
		if serviceSnapshot.Ready {
			anyServiceReady = true
		}

		if service.Required && !serviceSnapshot.Ready {
			allRequiredReady = false
			if firstRequiredFailure == "" {
				switch {
				case service.State != runtime.ServiceStateRunning:
					firstRequiredFailure = ReasonRequiredServiceNotRunning
				case !serviceSnapshot.Attached:
					firstRequiredFailure = ReasonRequiredServiceDisconnected
				case !serviceSnapshot.Connected:
					firstRequiredFailure = ReasonRequiredServiceStale
				default:
					firstRequiredFailure = ReasonRequiredServiceNotReady
				}
			}
		}

		snapshot.Services = append(snapshot.Services, serviceSnapshot)
	}

	switch {
	case !snapshot.Operational:
		snapshot.Reason = ReasonRuntimeNotOperational
	case generation <= 0:
		snapshot.Reason = ReasonNoActiveGeneration
	case len(snapshot.Services) == 0:
		snapshot.Reason = ReasonTopologyEmpty
	case snapshot.RequiredServiceCount > 0 && !allRequiredReady:
		snapshot.Reason = firstRequiredFailure
	case snapshot.RequiredServiceCount == 0 && !anyServiceReady:
		// A descriptor with only optional services must not become vacuously
		// ready when no service has completed a current-generation handshake.
		snapshot.Reason = ReasonNoServiceReady
	default:
		snapshot.Ready = true
		snapshot.Reason = ReasonReady
	}

	return snapshot, nil
}

func (r *Resolver) IsReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	snapshot, err := r.Resolve(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	return snapshot.Ready, nil
}

func (r *Resolver) IsServiceReady(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (bool, error) {
	snapshot, err := r.Resolve(ctx, runtimeID)
	if err != nil {
		return false, err
	}
	service, found := snapshot.Service(serviceID)
	if !found {
		return false, nil
	}
	return service.Ready, nil
}

var _ Reader = (*Resolver)(nil)
