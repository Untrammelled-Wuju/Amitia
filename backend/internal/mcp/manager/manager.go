// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/auth"
	"github.com/u-ai/backend/internal/mcp/client"
	"github.com/u-ai/backend/internal/mcp/protocol"
	"github.com/u-ai/backend/internal/mcp/transport"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
)

type OAuthTokens interface {
	AccessToken(context.Context, string) (string, error)
}

type Factory interface {
	Build(context.Context, mcp.Server) (transport.MCPTransport, error)
}

type DefaultFactory struct {
	Repository *mcp.Repository
	Secrets    auth.SecretStore
	OAuth      OAuthTokens
	Commands   commandenv.Resolver
}

func (f DefaultFactory) Build(ctx context.Context, server mcp.Server) (transport.MCPTransport, error) {
	switch server.Transport {
	case "streamable_http":
		headers, err := f.httpHeaders(ctx, server)
		if err != nil {
			return nil, err
		}
		allowPrivate := false
		if f.Repository != nil {
			allowPrivate, _, _ = f.Repository.ServerCapabilityEnabled(ctx, server.ID, "private_network")
		}
		return transport.NewStreamableHTTP(transport.HTTPConfig{Endpoint: server.Endpoint, Headers: headers, Timeout: 30 * time.Second, MaxMessageBytes: 4 << 20, Policy: transport.EndpointPolicy{AllowLoopback: true, AllowPrivate: allowPrivate, MaxRedirects: 3}}), nil
	case "stdio":
		var args []string
		if err := json.Unmarshal([]byte(server.ArgsJSON), &args); err != nil {
			return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: args")
		}
		if f.Commands != nil {
			req := commandenv.Request{Command: server.Command, Args: args}
			inv, err := f.Commands.Resolve(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: command resolve failed: %w", err)
			}
			environment, err := f.stdioEnvironment(ctx, server)
			if err != nil {
				return nil, err
			}
			cfg := transport.StdioConfig{
				Executable:     inv.Executable,
				Args:           inv.Args,
				OriginalCommand: server.Command,
				WorkDir:        server.WorkDir,
				Environment:    environment,
				StartTimeout:   10 * time.Second,
				ShutdownTimeout: 3 * time.Second,
				MaxMessageBytes: 4 << 20,
			}
			return transport.NewStdio(cfg), nil
		}
		environment, err := f.stdioEnvironment(ctx, server)
		if err != nil {
			return nil, err
		}
		return transport.NewStdio(transport.StdioConfig{
			Command:         server.Command,
			Args:            args,
			WorkDir:         server.WorkDir,
			Environment:     environment,
			StartTimeout:    10 * time.Second,
			ShutdownTimeout: 3 * time.Second,
			MaxMessageBytes: 4 << 20,
		}), nil
	default:
		return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: transport")
	}
}

func (f DefaultFactory) httpHeaders(ctx context.Context, server mcp.Server) (map[string]string, error) {
	headers := map[string]string{}
	switch server.AuthType {
	case "", "none":
	case "oauth":
		if f.OAuth == nil {
			return nil, fmt.Errorf("MCP_AUTH_REQUIRED")
		}
		token, err := f.OAuth.AccessToken(ctx, server.ID)
		if err != nil {
			return nil, err
		}
		headers["Authorization"] = "Bearer " + token
	case "bearer_token":
		value, err := f.resolveCredential(ctx, server.ID, "bearer_token")
		if err != nil {
			return nil, err
		}
		headers["Authorization"] = "Bearer " + string(value)
	case "custom_headers":
		value, err := f.resolveCredential(ctx, server.ID, "custom_headers")
		if err != nil {
			return nil, err
		}
		var configured map[string]string
		if json.Unmarshal(value, &configured) != nil {
			return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: custom headers")
		}
		for key, value := range configured {
			lower := strings.ToLower(strings.TrimSpace(key))
			if lower == "host" || lower == "content-length" || lower == "origin" || strings.HasPrefix(lower, "mcp-") || strings.ContainsAny(key+value, "\r\n") {
				return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: restricted header")
			}
			headers[key] = value
		}
	default:
		return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: auth type")
	}
	return headers, nil
}

func (f DefaultFactory) stdioEnvironment(ctx context.Context, server mcp.Server) (map[string]string, error) {
	if server.AuthType != "stdio_env" {
		return map[string]string{}, nil
	}
	value, err := f.resolveCredential(ctx, server.ID, "stdio_env")
	if err != nil {
		return nil, err
	}
	var environment map[string]string
	if json.Unmarshal(value, &environment) != nil {
		return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: stdio env")
	}
	return environment, nil
}

func (f DefaultFactory) resolveCredential(ctx context.Context, serverID, credentialType string) ([]byte, error) {
	if f.Repository == nil || f.Secrets == nil {
		return nil, fmt.Errorf("MCP_AUTH_REQUIRED")
	}
	record, err := f.Repository.CredentialReference(ctx, serverID, credentialType)
	if err != nil {
		return nil, fmt.Errorf("MCP_AUTH_REQUIRED")
	}
	return f.Secrets.Get(ctx, record.SecretReference)
}

type Config struct {
	MaxReconnectAttempts int
	Backoff              []time.Duration
	Connection           client.Config
}

type Manager struct {
	repository    *mcp.Repository
	factory       Factory
	config        Config
	mu            sync.RWMutex
	connections   map[string]*client.Connection
	reconnecting  map[string]bool
	readyHandlers []func(context.Context, string)
	closed        bool
	root          context.Context
	cancel        context.CancelFunc
}

func (m *Manager) RegisterReadyHandler(handler func(context.Context, string)) {
	if handler == nil {
		return
	}
	m.mu.Lock()
	m.readyHandlers = append(m.readyHandlers, handler)
	m.mu.Unlock()
}

func New(repository *mcp.Repository, factory Factory, config Config) *Manager {
	if config.MaxReconnectAttempts <= 0 {
		config.MaxReconnectAttempts = 6
	}
	if len(config.Backoff) == 0 {
		config.Backoff = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}
	}
	root, cancel := context.WithCancel(context.Background())
	return &Manager{repository: repository, factory: factory, config: config, connections: map[string]*client.Connection{}, reconnecting: map[string]bool{}, root: root, cancel: cancel}
}

func (m *Manager) Restore(ctx context.Context) error {
	servers, err := m.repository.ListEnabledServers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		server := server
		go func() {
			if err := m.connect(m.root, server); err != nil {
				m.scheduleReconnect(server.ID, 1)
			}
		}()
	}
	return nil
}

func (m *Manager) Connect(ctx context.Context, serverID string) error {
	server, err := m.repository.GetServer(ctx, serverID)
	if err != nil {
		return err
	}
	return m.connect(ctx, server)
}

func (m *Manager) connect(ctx context.Context, server mcp.Server) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return fmt.Errorf("MCP manager closed")
	}
	existing := m.connections[server.ID]
	m.mu.Unlock()
	if existing != nil && existing.State() == client.StateReady {
		return nil
	}
	_ = m.repository.SetServerStatus(context.Background(), server.ID, "connecting", "", "", nil)
	target, err := m.factory.Build(ctx, server)
	if err != nil {
		m.recordFailure(server.ID, err)
		return err
	}
	connectionConfig := m.config.Connection
	connectionConfig.Capabilities = m.clientCapabilities(ctx, server.ID)
	connection := client.NewConnection(target, connectionConfig)
	if err := connection.Connect(ctx); err != nil {
		m.recordFailure(server.ID, err)
		return err
	}
	initialized := connection.InitializeResult()
	serverInfo, _ := json.Marshal(initialized.ServerInfo)
	capabilities, _ := json.Marshal(initialized.Capabilities)
	persisted := &struct{ ProtocolVersion, ServerInfoJSON, CapabilitiesJSON, Instructions string }{initialized.ProtocolVersion, string(serverInfo), string(capabilities), initialized.Instructions}
	if err := m.repository.SetServerStatus(context.Background(), server.ID, "ready", "", "", persisted); err != nil {
		_ = connection.Close(context.Background())
		return err
	}
	m.mu.Lock()
	old := m.connections[server.ID]
	m.connections[server.ID] = connection
	m.reconnecting[server.ID] = false
	handlers := append([]func(context.Context, string){}, m.readyHandlers...)
	m.mu.Unlock()
	if old != nil && old != connection {
		_ = old.Close(context.Background())
	}
	go func() {
		<-connection.Done()
		if connection.State() != client.StateStopping {
			m.scheduleReconnect(server.ID, 1)
		}
	}()
	for _, handler := range handlers {
		go handler(m.root, server.ID)
	}
	return nil
}

func (m *Manager) clientCapabilities(ctx context.Context, serverID string) protocol.ClientCapabilities {
	configured := m.config.Connection.Capabilities
	capabilities := protocol.ClientCapabilities{}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "roots"); err == nil && enabled {
		capabilities.Roots = configured.Roots
		if capabilities.Roots == nil {
			capabilities.Roots = map[string]any{"listChanged": true}
		}
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "sampling"); err == nil && enabled {
		capabilities.Sampling = configured.Sampling
		if capabilities.Sampling == nil {
			capabilities.Sampling = map[string]any{}
		}
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "elicitation"); err == nil && enabled {
		capabilities.Elicitation = configured.Elicitation
		if capabilities.Elicitation == nil {
			capabilities.Elicitation = map[string]any{}
		}
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "tasks"); err == nil && enabled {
		capabilities.Tasks = configured.Tasks
		if capabilities.Tasks == nil {
			capabilities.Tasks = map[string]any{}
		}
	}
	return capabilities
}

func (m *Manager) Disconnect(ctx context.Context, serverID string) error {
	m.mu.Lock()
	connection := m.connections[serverID]
	delete(m.connections, serverID)
	m.reconnecting[serverID] = false
	m.mu.Unlock()
	if connection != nil {
		if err := connection.Close(ctx); err != nil {
			return err
		}
	}
	return m.repository.SetServerStatus(ctx, serverID, "disconnected", "", "", nil)
}

func (m *Manager) Reconnect(ctx context.Context, serverID string) error {
	if err := m.Disconnect(ctx, serverID); err != nil {
		return err
	}
	return m.Connect(ctx, serverID)
}

func (m *Manager) Connection(serverID string) (*client.Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connection, ok := m.connections[serverID]
	return connection, ok && connection.State() == client.StateReady
}

func (m *Manager) Call(ctx context.Context, serverID, method string, params any, options client.CallOptions) (json.RawMessage, error) {
	connection, ok := m.Connection(serverID)
	if !ok {
		return nil, fmt.Errorf("MCP_SERVER_NOT_READY")
	}
	result, err := connection.Call(ctx, method, params, options)
	if err != nil {
		var rpcError *protocol.RPCError
		if !errors.As(err, &rpcError) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			m.scheduleReconnect(serverID, 1)
		}
	}
	return result, err
}

func (m *Manager) Close(ctx context.Context) error {
	m.cancel()
	m.mu.Lock()
	m.closed = true
	connections := make([]*client.Connection, 0, len(m.connections))
	for _, connection := range m.connections {
		connections = append(connections, connection)
	}
	m.connections = map[string]*client.Connection{}
	m.mu.Unlock()
	var first error
	for _, connection := range connections {
		if err := connection.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *Manager) scheduleReconnect(serverID string, attempt int) {
	m.mu.Lock()
	if m.closed || m.reconnecting[serverID] {
		m.mu.Unlock()
		return
	}
	m.reconnecting[serverID] = true
	m.mu.Unlock()
	go func() {
		defer func() { m.mu.Lock(); m.reconnecting[serverID] = false; m.mu.Unlock() }()
		for current := attempt; current <= m.config.MaxReconnectAttempts; current++ {
			delay := m.config.Backoff[min(current-1, len(m.config.Backoff)-1)]
			select {
			case <-time.After(delay):
			case <-m.root.Done():
				return
			}
			server, err := m.repository.GetServer(m.root, serverID)
			if err != nil || server.Enabled != 1 {
				return
			}
			if err := m.connect(m.root, server); err == nil {
				return
			}
		}
		_ = m.repository.SetServerStatus(context.Background(), serverID, "degraded", "MCP_RECONNECT_LIMIT_REACHED", "reconnect limit reached", nil)
	}()
}

func (m *Manager) recordFailure(serverID string, err error) {
	code := "MCP_TRANSPORT_START_FAILED"
	message := strings.TrimSpace(err.Error())
	if index := strings.Index(message, ":"); index > 0 && strings.HasPrefix(message[:index], "MCP_") {
		code = message[:index]
	}
	_ = m.repository.SetServerStatus(context.Background(), serverID, "disconnected", code, message, nil)
}
