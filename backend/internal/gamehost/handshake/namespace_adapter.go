package handshake

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type NamespaceApplyResult struct {
	Registered []string
	Reused     []string
}

type NamespaceAdapter interface {
	Apply(ctx context.Context, connectionID, pluginID, runtimeID, serviceID string, namespaces []string) (*NamespaceApplyResult, error)
	RemoveConnection(ctx context.Context, runtimeID, serviceID, connectionID string) error
}

type rpcNamespaceAdapter struct {
	registry rpc.NamespaceRegistry
}

func NewNamespaceAdapter(registry rpc.NamespaceRegistry) NamespaceAdapter {
	return &rpcNamespaceAdapter{
		registry: registry,
	}
}

func (a *rpcNamespaceAdapter) Apply(
	ctx context.Context,
	connectionID string,
	pluginID string,
	runtimeID string,
	serviceID string,
	namespaces []string,
) (*NamespaceApplyResult, error) {
	result := &NamespaceApplyResult{
		Registered: make([]string, 0),
		Reused:     make([]string, 0),
	}

	for _, ns := range namespaces {
		if err := rpc.ValidateCustomNamespace(rpc.Namespace(ns)); err != nil {
			return nil, NewHandshakeError(
				HandshakeErrorNamespaceInvalid,
				domain.ErrInvalidArgument,
				"invalid namespace: "+ns+": "+err.Error(),
			)
		}
	}

	rid := domain.RuntimeInstanceID(runtimeID)
	sid := domain.ServiceID(serviceID)

	nsSlice := make([]rpc.Namespace, 0, len(namespaces))
	for _, ns := range namespaces {
		nsSlice = append(nsSlice, rpc.Namespace(ns))
	}

	reconcileResult, err := a.registry.ReconcileService(ctx, domain.PluginID(pluginID), rid, sid, connectionID, nsSlice)
	if err != nil {
		return nil, NewHandshakeError(
			HandshakeErrorNamespaceConflict,
			domain.ErrInternal,
			"namespace reconciliation failed: "+err.Error(),
		)
	}
	for _, ns := range reconcileResult.Registered {
		result.Registered = append(result.Registered, string(ns))
	}
	for _, ns := range reconcileResult.Reused {
		result.Reused = append(result.Reused, string(ns))
	}

	return result, nil
}

func (a *rpcNamespaceAdapter) RemoveConnection(
	ctx context.Context,
	runtimeID string,
	serviceID string,
	connectionID string,
) error {
	return a.registry.UnregisterByConnection(ctx, domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID), connectionID)
}
