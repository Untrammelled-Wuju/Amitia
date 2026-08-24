package hostapi

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
)

type ProductionHostAPIAdapterDeps struct {
	Gateway            host_api.Gateway
	PluginRegistry     contracts.PluginRegistry
	RuntimeReader      RuntimeStateReader
	TopologyReader     RuntimeTopologyReader
	ReadyGate          *handshake.ReadyGate
	PermissionProvider PermissionSnapshotIDProvider
	ScopeProvider      ScopeSnapshotIDProvider
	RuntimeResolver    RuntimeResolver
	GenerationReader   RuntimeGenerationReader
	ConnectionRegistry ConnectionReadyChecker
	InvocationTracker  InvocationTracker
	PermissionChecker  RuntimePermissionChecker
	FeatureChecker     NegotiatedFeatureChecker
}

type RuntimeGenerationReader interface {
	CurrentGeneration(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, error)
	AllocateGeneration(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, error)
}

type ConnectionReadyChecker interface {
	IsReady(connKey string) bool
}

func NewProductionHostAPIAdapter(deps ProductionHostAPIAdapterDeps) (*HostAPIAdapter, error) {
	if deps.Gateway == nil {
		return nil, fmt.Errorf("hostapi: gateway is required")
	}
	if deps.PluginRegistry == nil {
		return nil, fmt.Errorf("hostapi: plugin registry is required")
	}
	if deps.RuntimeReader == nil {
		return nil, fmt.Errorf("hostapi: runtime reader is required")
	}
	if deps.TopologyReader == nil {
		return nil, fmt.Errorf("hostapi: topology reader is required")
	}
	if deps.ReadyGate == nil {
		return nil, fmt.Errorf("hostapi: ready gate is required")
	}
	if deps.PermissionProvider == nil {
		return nil, fmt.Errorf("hostapi: permission snapshot provider is required")
	}
	if deps.ScopeProvider == nil {
		return nil, fmt.Errorf("hostapi: scope snapshot provider is required")
	}
	if deps.RuntimeResolver == nil {
		return nil, fmt.Errorf("hostapi: runtime resolver is required")
	}
	if deps.GenerationReader == nil {
		return nil, fmt.Errorf("hostapi: generation reader is required")
	}
	if deps.ConnectionRegistry == nil {
		return nil, fmt.Errorf("hostapi: connection registry is required")
	}

	identityMapper := NewIdentityMapper(
		deps.RuntimeReader,
		deps.TopologyReader,
		deps.RuntimeResolver,
	)

	return NewHostAPIAdapter(HostAPIAdapterConfig{
		Gateway:            deps.Gateway,
		Mapper:             identityMapper,
		PermissionProvider: deps.PermissionProvider,
		ScopeProvider:      deps.ScopeProvider,
		ReadyVerifier:      deps.ConnectionRegistry,
		IDGenerator:        DefaultIDGenerator(),
		InvocationTracker:  deps.InvocationTracker,
		PermissionChecker:  deps.PermissionChecker,
		FeatureChecker:     deps.FeatureChecker,
	})
}
