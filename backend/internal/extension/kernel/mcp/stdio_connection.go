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
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
)

// CanonicalStdioConnection is the Extension Kernel owned MCP stdio connection.
// It wraps Legacy MCP transport and client to provide canonical interface.
type CanonicalStdioConnection struct {
	spec       MCPStdioResolvedSpec
	mu         sync.RWMutex
	state      MCPStdioServerState
	transport  transport.MCPTransport
	connection *client.Connection
}

// CanonicalStdioFactory creates CanonicalStdioConnection instances.
type CanonicalStdioFactory struct {
	commandResolver commandenv.Resolver
}

// NewCanonicalStdioFactory creates a new factory with the given command resolver.
func NewCanonicalStdioFactory(resolver commandenv.Resolver) *CanonicalStdioFactory {
	return &CanonicalStdioFactory{commandResolver: resolver}
}

// Create builds a new CanonicalStdioConnection from the given spec.
func (f *CanonicalStdioFactory) Create(ctx context.Context, spec MCPStdioSpec) (*CanonicalStdioConnection, error) {
	resolved, err := f.resolveSpec(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &CanonicalStdioConnection{
		spec:  resolved,
		state: MCPStdioStateStopped,
	}, nil
}

func (f *CanonicalStdioFactory) resolveSpec(ctx context.Context, spec MCPStdioSpec) (MCPStdioResolvedSpec, error) {
	if f.commandResolver == nil {
		return MCPStdioResolvedSpec{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: command resolver not configured")
	}
	req := commandenv.Request{Command: spec.Command, Args: spec.Args}
	inv, err := f.commandResolver.Resolve(ctx, req)
	if err != nil {
		return MCPStdioResolvedSpec{}, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: command resolve failed: %w", err)
	}
	return MCPStdioResolvedSpec{
		ServerID:   spec.ServerID,
		Executable: inv.Executable,
		Args:       inv.Args,
		WorkDir:    spec.WorkDir,
		Env:        spec.Env,
	}, nil
}

// ServerID returns the unique server identifier.
func (c *CanonicalStdioConnection) ServerID() string {
	return c.spec.ServerID
}

// State returns the current connection state.
func (c *CanonicalStdioConnection) State() MCPStdioServerState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *CanonicalStdioConnection) setState(state MCPStdioServerState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

// Start initializes the stdio connection including process start and MCP handshake.
func (c *CanonicalStdioConnection) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.state != MCPStdioStateStopped && c.state != MCPStdioStateFailed {
		c.mu.Unlock()
		return fmt.Errorf("MCP server already started")
	}
	c.state = MCPStdioStateStarting
	c.mu.Unlock()

	startTimeout := c.spec.StartTimeout()
	if startTimeout <= 0 {
		startTimeout = 10 * time.Second
	}

	startCtx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()

	tr, err := c.buildTransport()
	if err != nil {
		c.setState(MCPStdioStateFailed)
		return err
	}

	if err := tr.Start(startCtx); err != nil {
		c.setState(MCPStdioStateFailed)
		return fmt.Errorf("MCP_TRANSPORT_START_FAILED: %w", err)
	}

	c.mu.Lock()
	c.transport = tr
	c.mu.Unlock()

	if err := c.performHandshake(startCtx); err != nil {
		_ = c.Close(ctx)
		return err
	}

	c.setState(MCPStdioStateReady)
	return nil
}

func (c *CanonicalStdioConnection) buildTransport() (transport.MCPTransport, error) {
	env := buildMinimalEnvironment(c.spec.Env)
	cfg := transport.StdioConfig{
		Executable:      c.spec.Executable,
		Args:            c.spec.Args,
		OriginalCommand: c.spec.Executable,
		WorkDir:         c.spec.WorkDir,
		Environment:     env,
		StartTimeout:    c.spec.StartTimeout(),
		ShutdownTimeout: c.spec.ShutdownTimeout(),
		MaxMessageBytes: c.spec.MaxMessageBytes(),
	}
	return transport.NewStdio(cfg), nil
}

func (c *CanonicalStdioConnection) performHandshake(ctx context.Context) error {
	c.setState(MCPStdioStateInitializing)

	c.mu.RLock()
	tr := c.transport
	c.mu.RUnlock()

	conn := client.NewConnection(tr, client.Config{
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
	return nil
}

// Close shuts down the stdio connection and terminates the process.
func (c *CanonicalStdioConnection) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.state == MCPStdioStateStopped {
		c.mu.Unlock()
		return nil
	}
	c.state = MCPStdioStateClosing
	conn := c.connection
	tr := c.transport
	c.mu.Unlock()

	shutdownTimeout := c.spec.ShutdownTimeout()
	if shutdownTimeout <= 0 {
		shutdownTimeout = 3 * time.Second
	}

	closeCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	var closeErr error
	if conn != nil {
		closeErr = conn.Close(closeCtx)
	}
	if tr != nil {
		_ = tr.Close(closeCtx)
	}

	c.mu.Lock()
	c.connection = nil
	c.transport = nil
	c.mu.Unlock()

	c.setState(MCPStdioStateStopped)
	return closeErr
}

// Call sends a JSON-RPC request to the MCP server.
func (c *CanonicalStdioConnection) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.RLock()
	conn := c.connection
	state := c.state
	c.mu.RUnlock()

	if state != MCPStdioStateReady || conn == nil {
		return nil, fmt.Errorf("MCP server not ready: %s", state)
	}
	return conn.Call(ctx, method, params, client.CallOptions{})
}

// ListTools retrieves the list of tools from the MCP server.
func (c *CanonicalStdioConnection) ListTools(ctx context.Context) ([]discovery.Tool, error) {
	result, err := c.Call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var response struct {
		Tools []discovery.Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("MCP_TOOL_LIST_INVALID: %w", err)
	}
	return response.Tools, nil
}

// Health returns the current health status.
func (c *CanonicalStdioConnection) Health() capability.HealthStatus {
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

// GetCaller returns the MCPCallFunc for this connection.
func (c *CanonicalStdioConnection) GetCaller() capability.MCPCallFunc {
	return c.callTool
}

// GetHealthFunc returns the MCPHealthFunc for this connection.
func (c *CanonicalStdioConnection) GetHealthFunc() capability.MCPHealthFunc {
	return c.healthCheck
}

func (c *CanonicalStdioConnection) callTool(ctx context.Context, serverID string, toolName string, input json.RawMessage) (json.RawMessage, error) {
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

func (c *CanonicalStdioConnection) healthCheck(ctx context.Context, serverID string) capability.HealthStatus {
	return c.Health()
}

func (spec MCPStdioResolvedSpec) StartTimeout() time.Duration {
	return 10 * time.Second
}

func (spec MCPStdioResolvedSpec) ShutdownTimeout() time.Duration {
	return 3 * time.Second
}

func (spec MCPStdioResolvedSpec) MaxMessageBytes() int64 {
	return 4 << 20
}

func buildMinimalEnvironment(explicit map[string]string) map[string]string {
	envMap := map[string]string{
		"PATH":  getEnv("PATH"),
		"HOME":  getEnv("HOME"),
		"TEMP":  getEnv("TEMP"),
		"TMP":   getEnv("TMP"),
	}
	for k, v := range explicit {
		envMap[k] = v
	}
	return envMap
}
