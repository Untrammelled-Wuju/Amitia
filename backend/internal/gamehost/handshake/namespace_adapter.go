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
	Apply(ctx context.Context, pluginID, runtimeID, serviceID string, namespaces []string) (*NamespaceApplyResult, error)
}

type rpcNamespaceAdapter struct {
	registry    rpc.NamespaceRegistry
	reservedSet map[string]struct{}
}

func NewNamespaceAdapter(registry rpc.NamespaceRegistry) NamespaceAdapter {
	reserved := map[string]struct{}{
		"host":    {},
		"plugin":  {},
		"runtime": {},
		"service": {},
		"control": {},
		"channel": {},
	}
	return &rpcNamespaceAdapter{
		registry:    registry,
		reservedSet: reserved,
	}
}

func (a *rpcNamespaceAdapter) Apply(
	ctx context.Context,
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
		if _, ok := a.reservedSet[ns]; ok {
			return nil, NewHandshakeError(
				HandshakeErrorNamespaceInvalid,
				domain.ErrInvalidArgument,
				"reserved namespace cannot be registered: "+ns,
			)
		}

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

	currentList, err := a.registry.List(ctx, rid)
	if err != nil {
		return nil, NewHandshakeError(
			HandshakeErrorNamespaceInvalid,
			domain.ErrInternal,
			"failed to list existing namespaces",
		)
	}
	currentMap := make(map[string]string, len(currentList))
	for _, route := range currentList {
		currentMap[string(route.Namespace)] = string(route.ServiceID)
	}

	for _, ns := range namespaces {
		if existing, ok := currentMap[ns]; ok {
			if existing == serviceID {
				result.Reused = append(result.Reused, ns)
				continue
			}
			return nil, NewHandshakeError(
				HandshakeErrorNamespaceConflict,
				domain.ErrAlreadyExists,
				"namespace already owned by another service: "+ns+" (owner: "+existing+")",
			)
		}
	}

	for _, ns := range namespaces {
		err := a.registry.Register(ctx, rpc.Route{
			PluginID:  domain.PluginID(pluginID),
			RuntimeID: rid,
			ServiceID: sid,
			Namespace: rpc.Namespace(ns),
		})
		if err != nil {
			for _, done := range result.Registered {
				_ = a.registry.Unregister(ctx, rid, rpc.Namespace(done))
			}
			return nil, NewHandshakeError(
				HandshakeErrorNamespaceConflict,
				domain.ErrInternal,
				"namespace registration failed: "+ns+": "+err.Error(),
			)
		}
		result.Registered = append(result.Registered, ns)
	}

	return result, nil
}
