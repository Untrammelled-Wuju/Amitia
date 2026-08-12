// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"fmt"
	"sync"
)

// CanonicalStdioRegistry manages CanonicalStdioConnection instances.
// It enforces single-owner guard: same serverId allows only one active connection.
type CanonicalStdioRegistry struct {
	mu          sync.Mutex
	connections map[string]*CanonicalStdioConnection
	factory     *CanonicalStdioFactory
}

// NewCanonicalStdioRegistry creates a new registry with the given factory.
func NewCanonicalStdioRegistry(factory *CanonicalStdioFactory) *CanonicalStdioRegistry {
	return &CanonicalStdioRegistry{
		connections: make(map[string]*CanonicalStdioConnection),
		factory:     factory,
	}
}

// StartOrGet starts a new connection or returns existing one for the given spec.
// Enforces single-owner guard: same serverId allows only one active connection.
func (r *CanonicalStdioRegistry) StartOrGet(ctx context.Context, spec MCPStdioSpec) (*CanonicalStdioConnection, error) {
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

// Get returns the connection for the given serverId.
func (r *CanonicalStdioRegistry) Get(serverID string) (*CanonicalStdioConnection, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conn, ok := r.connections[serverID]
	return conn, ok
}

// Close closes the connection for the given serverId.
func (r *CanonicalStdioRegistry) Close(ctx context.Context, serverID string) error {
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

// CloseAll closes all managed connections.
func (r *CanonicalStdioRegistry) CloseAll(ctx context.Context) error {
	r.mu.Lock()
	conns := make(map[string]*CanonicalStdioConnection, len(r.connections))
	for k, v := range r.connections {
		conns[k] = v
	}
	r.connections = make(map[string]*CanonicalStdioConnection)
	r.mu.Unlock()

	var lastErr error
	for _, conn := range conns {
		if err := conn.Close(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// List returns all managed connections.
func (r *CanonicalStdioRegistry) List() map[string]*CanonicalStdioConnection {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[string]*CanonicalStdioConnection, len(r.connections))
	for k, v := range r.connections {
		result[k] = v
	}
	return result
}

// RegisterLegacyOwnership marks a serverId as owned by Legacy Manager.
// This prevents double-start during migration.
func (r *CanonicalStdioRegistry) RegisterLegacyOwnership(serverID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.connections[serverID]; exists {
		return fmt.Errorf("MCP server already owned by Kernel: %s", serverID)
	}
	return nil
}

// IsOwnedByLegacy checks if a serverId is owned by Legacy Manager.
func (r *CanonicalStdioRegistry) IsOwnedByLegacy(serverID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.connections[serverID]
	return !exists
}
