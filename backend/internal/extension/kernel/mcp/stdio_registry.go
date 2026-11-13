package mcp

import "context"

type CanonicalStdioRegistry struct {
	registry *connectionRegistry[*CanonicalStdioConnection]
	factory  *CanonicalStdioFactory
}

func NewCanonicalStdioRegistry(factory *CanonicalStdioFactory) *CanonicalStdioRegistry {
	return &CanonicalStdioRegistry{registry: newConnectionRegistry[*CanonicalStdioConnection](), factory: factory}
}

func (r *CanonicalStdioRegistry) StartOrGet(ctx context.Context, spec MCPStdioSpec) (*CanonicalStdioConnection, error) {
	return r.registry.start(ctx, spec.ServerID, func(ctx context.Context) (*CanonicalStdioConnection, error) {
		return r.factory.Create(ctx, spec)
	})
}

func (r *CanonicalStdioRegistry) Get(serverID string) (*CanonicalStdioConnection, bool) {
	return r.registry.get(serverID)
}

func (r *CanonicalStdioRegistry) Close(ctx context.Context, serverID string) error {
	return r.registry.close(ctx, serverID, nil)
}

func (r *CanonicalStdioRegistry) CloseAll(ctx context.Context) error {
	return r.registry.closeAll(ctx)
}

func (r *CanonicalStdioRegistry) List() map[string]*CanonicalStdioConnection {
	return r.registry.list()
}
