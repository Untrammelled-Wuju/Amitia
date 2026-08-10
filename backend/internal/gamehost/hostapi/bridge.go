package hostapi

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type bridgeRuntimeStateReader struct {
	runtimes contracts.RuntimeManager
	plugins  contracts.PluginRegistry
}

func NewBridgeRuntimeStateReader(
	runtimes contracts.RuntimeManager,
	plugins contracts.PluginRegistry,
) RuntimeStateReader {
	return &bridgeRuntimeStateReader{runtimes: runtimes, plugins: plugins}
}

func (b *bridgeRuntimeStateReader) RuntimeExists(ctx context.Context, runtimeID string) (bool, error) {
	if b.runtimes == nil {
		return false, fmt.Errorf("bridge runtime reader: runtime store not configured")
	}
	_, err := b.runtimes.Get(ctx, domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (b *bridgeRuntimeStateReader) RuntimeOwnedBy(ctx context.Context, runtimeID string, pluginID string) (bool, error) {
	if b.runtimes == nil {
		return false, fmt.Errorf("bridge runtime reader: runtime store not configured")
	}
	inst, err := b.runtimes.Get(ctx, domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return false, err
	}
	return inst.PluginID == domain.PluginID(pluginID), nil
}

func (b *bridgeRuntimeStateReader) PluginExists(ctx context.Context, pluginID string) (bool, error) {
	if b.plugins == nil {
		return false, fmt.Errorf("bridge runtime reader: plugin registry not configured")
	}
	_, err := b.plugins.Get(ctx, domain.PluginID(pluginID))
	return err == nil, nil
}

type bridgeTopologyReader struct {
	topology runtime.RuntimeTopologyStore
}

func NewBridgeTopologyReader(topology runtime.RuntimeTopologyStore) RuntimeTopologyReader {
	return &bridgeTopologyReader{topology: topology}
}

func (b *bridgeTopologyReader) ServiceBelongsToRuntime(ctx context.Context, runtimeID string, serviceID string) error {
	if b.topology == nil {
		return fmt.Errorf("bridge topology reader: topology store not configured")
	}
	snap, err := b.topology.GetTopologySnapshot(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return fmt.Errorf("get topology: %w", err)
	}
	for _, svc := range snap.Services {
		if string(svc.ServiceID) == serviceID {
			return nil
		}
	}
	return fmt.Errorf("service %q not found in runtime %q topology", serviceID, runtimeID)
}

type bridgeRuntimeInfoResolver struct{}

func (b *bridgeRuntimeInfoResolver) RuntimeInfo(
	ctx context.Context,
	runtimeID string,
) (extensionID string, moduleID string, runtimeType string, generation int64, err error) {
	return "", "", "", 0, fmt.Errorf("bridge runtime resolver requires kernel runtime metadata source")
}

type bridgeSnapshotProvider struct{}

func (b *bridgeSnapshotProvider) CurrentSnapshotID(
	ctx context.Context,
	extensionID string,
	moduleID string,
	generation int64,
) (snapshotID string, ok bool, err error) {
	return "", false, nil
}

type noopReadyVerifier struct {
	gate *handshake.ReadyGate
}

func (n *noopReadyVerifier) IsReady(connKey string) bool {
	if n.gate == nil {
		return true
	}
	return n.gate.IsReady(connKey)
}

type ProductionHostAPIAdapterDeps struct {
	Gateway            host_api.Gateway
	PluginRegistry     contracts.PluginRegistry
	RuntimeStore       contracts.RuntimeManager
	Topology           runtime.RuntimeTopologyStore
	ReadyGate          *handshake.ReadyGate
	PermissionProvider PermissionSnapshotIDProvider
	ScopeProvider      ScopeSnapshotIDProvider
	RuntimeResolver    RuntimeResolver
}

func NewProductionHostAPIAdapter(deps ProductionHostAPIAdapterDeps) (*HostAPIAdapter, error) {
	if deps.Gateway == nil {
		return nil, fmt.Errorf("hostapi: gateway is required")
	}
	if deps.ReadyGate == nil {
		return nil, fmt.Errorf("hostapi: ready gate is required")
	}

	stateReader := NewBridgeRuntimeStateReader(deps.RuntimeStore, deps.PluginRegistry)
	topologyReader := NewBridgeTopologyReader(deps.Topology)

	resolver := deps.RuntimeResolver
	if resolver == nil {
		resolver = &bridgeRuntimeInfoResolver{}
	}

	identityMapper := NewIdentityMapper(stateReader, topologyReader, resolver)

	permProv := deps.PermissionProvider
	if permProv == nil {
		permProv = &bridgeSnapshotProvider{}
	}

	scopeProv := deps.ScopeProvider
	if scopeProv == nil {
		scopeProv = &bridgeSnapshotProvider{}
	}

	return NewHostAPIAdapter(HostAPIAdapterConfig{
		Gateway:            deps.Gateway,
		Mapper:             identityMapper,
		PermissionProvider: permProv,
		ScopeProvider:      scopeProv,
		ReadyVerifier:      &noopReadyVerifier{gate: deps.ReadyGate},
		IDGenerator:        DefaultIDGenerator(),
	})
}
