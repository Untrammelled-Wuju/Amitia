package mcp

import "context"

type CanonicalRemoteRegistry struct {
	registry *connectionRegistry[*CanonicalRemoteConnection]
	factory  *CanonicalRemoteFactory
}

func NewCanonicalRemoteRegistry(factory *CanonicalRemoteFactory) *CanonicalRemoteRegistry {
	return &CanonicalRemoteRegistry{registry: newConnectionRegistry[*CanonicalRemoteConnection](), factory: factory}
}

func (r *CanonicalRemoteRegistry) StartOrGet(ctx context.Context, spec MCPRemoteSpec) (*CanonicalRemoteConnection, error) {
	return r.registry.start(ctx, spec.ServerID, func(ctx context.Context) (*CanonicalRemoteConnection, error) {
		return r.factory.Create(ctx, spec)
	})
}

func (r *CanonicalRemoteRegistry) Get(serverID string) (*CanonicalRemoteConnection, bool) {
	return r.registry.get(serverID)
}

func (r *CanonicalRemoteRegistry) Close(ctx context.Context, serverID string) error {
	return r.registry.close(ctx, serverID, nil)
}

func (r *CanonicalRemoteRegistry) CloseAll(ctx context.Context) error {
	return r.registry.closeAll(ctx)
}

func (r *CanonicalRemoteRegistry) List() map[string]*CanonicalRemoteConnection {
	return r.registry.list()
}
