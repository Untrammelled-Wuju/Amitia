package hostapi

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type IdentityMapper interface {
	MapIdentity(ctx context.Context, peer Peer) (runtime_supervisor.RuntimeIdentity, error)
}

type RuntimeResolver interface {
	RuntimeInfo(
		ctx context.Context,
		runtimeID string,
		serviceID string,
	) (extensionID string, moduleID string, runtimeType string, generation int64, err error)
}

type RuntimeTopologyReader interface {
	ServiceBelongsToRuntime(
		ctx context.Context,
		runtimeID string,
		serviceID string,
	) error
}

type RuntimeStateReader interface {
	RuntimeExists(ctx context.Context, runtimeID string) (bool, error)
	RuntimeOwnedBy(ctx context.Context, runtimeID string, pluginID string) (bool, error)
	PluginExists(ctx context.Context, pluginID string) (bool, error)
}

type defaultIdentityMapper struct {
	runtimeReader   RuntimeStateReader
	topologyReader  RuntimeTopologyReader
	runtimeResolver RuntimeResolver
}

func NewIdentityMapper(
	runtimeReader RuntimeStateReader,
	topologyReader RuntimeTopologyReader,
	runtimeResolver RuntimeResolver,
) IdentityMapper {
	return &defaultIdentityMapper{
		runtimeReader:   runtimeReader,
		topologyReader:  topologyReader,
		runtimeResolver: runtimeResolver,
	}
}

func (m *defaultIdentityMapper) MapIdentity(ctx context.Context, peer Peer) (runtime_supervisor.RuntimeIdentity, error) {
	if err := peer.Validate(); err != nil {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("peer invalid: %w", err)
	}

	if ok, err := m.runtimeReader.PluginExists(ctx, string(peer.PluginID)); err != nil {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("query plugin state: %w", err)
	} else if !ok {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("peer plugin %q not registered", peer.PluginID)
	}

	if ok, err := m.runtimeReader.RuntimeExists(ctx, string(peer.RuntimeID)); err != nil {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("query runtime state: %w", err)
	} else if !ok {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("runtime %q not found", peer.RuntimeID)
	}

	if owned, err := m.runtimeReader.RuntimeOwnedBy(ctx, string(peer.RuntimeID), string(peer.PluginID)); err != nil {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("query runtime ownership: %w", err)
	} else if !owned {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("runtime %q does not belong to plugin %q", peer.RuntimeID, peer.PluginID)
	}

	if err := m.topologyReader.ServiceBelongsToRuntime(ctx, string(peer.RuntimeID), string(peer.ServiceID)); err != nil {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("service %q does not belong to runtime %q: %w", peer.ServiceID, peer.RuntimeID, err)
	}

	extID, modID, rtType, gen, err := m.runtimeResolver.RuntimeInfo(ctx, string(peer.RuntimeID), string(peer.ServiceID))
	if err != nil {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("resolve runtime kernel info: %w", err)
	}
	if peer.Generation != gen {
		return runtime_supervisor.RuntimeIdentity{}, fmt.Errorf("peer generation %d does not match current runtime generation %d", peer.Generation, gen)
	}

	return runtime_supervisor.RuntimeIdentity{
		InstanceID:  string(peer.RuntimeID),
		ExtensionID: domain.ExtensionID(extID),
		ModuleID:    domain.ModuleID(modID),
		RuntimeType: domain.RuntimeType(rtType),
		Generation:  gen,
	}, nil
}
