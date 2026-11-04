package mcp

import (
	"context"
	"sync"
)

type CanonicalRemoteRegistry struct {
	mu          sync.Mutex
	connections map[string]*CanonicalRemoteConnection
	factory     *CanonicalRemoteFactory
}

func NewCanonicalRemoteRegistry(factory *CanonicalRemoteFactory) *CanonicalRemoteRegistry {
	return &CanonicalRemoteRegistry{
		connections: make(map[string]*CanonicalRemoteConnection),
		factory:     factory,
	}
}

func (r *CanonicalRemoteRegistry) StartOrGet(ctx context.Context, spec MCPRemoteSpec) (*CanonicalRemoteConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	serverID := spec.ServerID
	if existing, ok := r.connections[serverID]; ok {
		if existing.State() == MCPStdioStateReady {
			return existing, nil
		}
	}

	conn, err := r.factory.Create(ctx, spec)
	if err != nil {
		return nil, err
	}

	if err := conn.Start(ctx); err != nil {
		return nil, err
	}

	r.connections[serverID] = conn
	return conn, nil
}

func (r *CanonicalRemoteRegistry) Get(serverID string) (*CanonicalRemoteConnection, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conn, ok := r.connections[serverID]
	return conn, ok
}

func (r *CanonicalRemoteRegistry) Close(ctx context.Context, serverID string) error {
	r.mu.Lock()
	conn, ok := r.connections[serverID]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	delete(r.connections, serverID)
	r.mu.Unlock()

	return conn.Close(ctx)
}

func (r *CanonicalRemoteRegistry) CloseAll(ctx context.Context) error {
	r.mu.Lock()
	conns := make(map[string]*CanonicalRemoteConnection, len(r.connections))
	for k, v := range r.connections {
		conns[k] = v
	}
	r.connections = make(map[string]*CanonicalRemoteConnection)
	r.mu.Unlock()

	var lastErr error
	for _, conn := range conns {
		if err := conn.Close(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (r *CanonicalRemoteRegistry) List() map[string]*CanonicalRemoteConnection {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[string]*CanonicalRemoteConnection, len(r.connections))
	for k, v := range r.connections {
		result[k] = v
	}
	return result
}
