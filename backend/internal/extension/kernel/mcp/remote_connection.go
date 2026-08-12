// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/mcp/client"
	"github.com/u-ai/backend/internal/mcp/discovery"
	"github.com/u-ai/backend/internal/mcp/protocol"
	"github.com/u-ai/backend/internal/mcp/transport"
)

type CanonicalRemoteConnection struct {
	spec       MCPRemoteResolvedSpec
	mu         sync.RWMutex
	state      MCPStdioServerState
	transport  RemoteTransport
	policy     RemoteEndpointPolicy
	connection *client.Connection
}

type CanonicalRemoteFactory struct{}

func NewCanonicalRemoteFactory() *CanonicalRemoteFactory {
	return &CanonicalRemoteFactory{}
}

func (f *CanonicalRemoteFactory) Create(ctx context.Context, spec MCPRemoteSpec) (*CanonicalRemoteConnection, error) {
	policy := RemoteEndpointPolicy{
		AllowLoopback:      spec.AllowLoopback,
		AllowPrivate:       spec.AllowPrivate,
		AllowPublicHTTP:    spec.AllowPublicHTTP,
		AllowHostRedirects: false,
		MaxRedirects:       spec.MaxRedirects,
	}

	resolved := MCPRemoteResolvedSpec{
		ServerID:        spec.ServerID,
		Endpoint:        spec.Endpoint,
		Timeout:         spec.Timeout,
		MaxMessageBytes: spec.MaxMessageBytes,
		StaticHeaders:   spec.StaticHeaders,
	}

	return &CanonicalRemoteConnection{
		spec:   resolved,
		policy: policy,
		state:  MCPStdioStateStopped,
	}, nil
}

func (c *CanonicalRemoteConnection) ServerID() string {
	return c.spec.ServerID
}

func (c *CanonicalRemoteConnection) State() MCPStdioServerState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *CanonicalRemoteConnection) setState(state MCPStdioServerState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

func (c *CanonicalRemoteConnection) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.state != MCPStdioStateStopped && c.state != MCPStdioStateFailed {
		c.mu.Unlock()
		return fmt.Errorf("MCP remote server already started")
	}
	c.state = MCPStdioStateStarting
	c.mu.Unlock()

	transport := NewStreamableHTTP(c.spec, c.policy)

	if err := transport.Start(ctx); err != nil {
		c.setState(MCPStdioStateFailed)
		return err
	}

	c.mu.Lock()
	c.transport = transport
	c.mu.Unlock()

	if err := c.performHandshake(ctx); err != nil {
		_ = c.Close(ctx)
		return err
	}

	c.setState(MCPStdioStateReady)
	return nil
}

func (c *CanonicalRemoteConnection) performHandshake(ctx context.Context) error {
	c.setState(MCPStdioStateInitializing)

	c.mu.RLock()
	streamTransport := c.transport
	c.mu.RUnlock()

	transportAdapter := &remoteTransportLegacyAdapter{transport: streamTransport}

	conn := client.NewConnection(transportAdapter, client.Config{
		ClientInfo: protocol.Implementation{
			Name:    "amitia",
			Title:   "Amitia",
			Version: "1.0.0",
		},
		Capabilities: protocol.ClientCapabilities{
			Roots: map[string]any{"listChanged": true},
		},
	})

	if err := conn.Connect(ctx); err != nil {
		return fmt.Errorf("%w: %v", protocol.ErrInitialization, err)
	}

	c.mu.Lock()
	c.connection = conn
	c.mu.Unlock()

	if err := streamTransport.StartServerStream(ctx); err != nil {
		return fmt.Errorf("MCP_REMOTE_STREAM_FAILED: %w", err)
	}

	return nil
}

func (c *CanonicalRemoteConnection) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.state == MCPStdioStateStopped {
		c.mu.Unlock()
		return nil
	}
	c.state = MCPStdioStateClosing
	conn := c.connection
	transport := c.transport
	c.mu.Unlock()

	var closeErr error
	if conn != nil {
		closeErr = conn.Close(ctx)
	}
	if transport != nil {
		_ = transport.Close(ctx)
	}

	c.mu.Lock()
	c.connection = nil
	c.transport = nil
	c.mu.Unlock()

	c.setState(MCPStdioStateStopped)
	return closeErr
}

func (c *CanonicalRemoteConnection) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.RLock()
	conn := c.connection
	state := c.state
	c.mu.RUnlock()

	if state != MCPStdioStateReady || conn == nil {
		return nil, fmt.Errorf("MCP remote server not ready: %s", state)
	}
	return conn.Call(ctx, method, params, client.CallOptions{})
}

func (c *CanonicalRemoteConnection) ListTools(ctx context.Context) ([]discovery.Tool, error) {
	result, err := c.Call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var response struct {
		Tools []discovery.Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("MCP_REMOTE_TOOL_LIST_INVALID: %w", err)
	}
	return response.Tools, nil
}

func (c *CanonicalRemoteConnection) Health() capability.HealthStatus {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	switch state {
	case MCPStdioStateReady:
		return capability.HealthReady
	case MCPStdioStateFailed:
		return capability.HealthUnhealthy
	default:
		return capability.HealthUnknown
	}
}

func (c *CanonicalRemoteConnection) GetCaller() capability.MCPCallFunc {
	return c.callTool
}

func (c *CanonicalRemoteConnection) GetHealthFunc() capability.MCPHealthFunc {
	return c.healthCheck
}

func (c *CanonicalRemoteConnection) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.transport == nil {
		return ""
	}
	return c.transport.SessionID()
}

func (c *CanonicalRemoteConnection) callTool(ctx context.Context, serverID string, toolName string, input json.RawMessage) (json.RawMessage, error) {
	var arguments any
	if err := json.Unmarshal(input, &arguments); err != nil {
		arguments = map[string]any{}
	}
	result, err := c.Call(ctx, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *CanonicalRemoteConnection) healthCheck(ctx context.Context, serverID string) capability.HealthStatus {
	return c.Health()
}

// remoteTransportLegacyAdapter wraps RemoteTransport to implement transport.MCPTransport.
type remoteTransportLegacyAdapter struct {
	transport RemoteTransport
}

func (a *remoteTransportLegacyAdapter) Start(ctx context.Context) error {
	return a.transport.Start(ctx)
}

func (a *remoteTransportLegacyAdapter) Send(ctx context.Context, message protocol.Message) error {
	return a.transport.Send(ctx, message)
}

func (a *remoteTransportLegacyAdapter) Receive() <-chan protocol.Message {
	return a.transport.Receive()
}

func (a *remoteTransportLegacyAdapter) Close(ctx context.Context) error {
	return a.transport.Close(ctx)
}

func (a *remoteTransportLegacyAdapter) State() transport.State {
	switch a.transport.State() {
	case RemoteStateStopped:
		return transport.StateStopped
	case RemoteStateStarting:
		return transport.StateStarting
	case RemoteStateRunning:
		return transport.StateRunning
	case RemoteStateClosing:
		return transport.StateClosing
	case RemoteStateError:
		return transport.StateError
	default:
		return transport.StateStopped
	}
}

func (spec MCPRemoteResolvedSpec) StartTimeout() time.Duration {
	return 15 * time.Second
}
