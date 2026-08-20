package canonical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/auth"
	"github.com/u-ai/backend/internal/mcp/client"
	"github.com/u-ai/backend/internal/mcp/protocol"
)

// OAuthTokens is the minimum OAuth capability required by the canonical MCP
// compatibility connection manager.
type OAuthTokens interface {
	AccessToken(context.Context, string) (string, error)
}

// Manager provides the existing /api/mcp connection contract while delegating
// all transport ownership to Extension Kernel canonical registries. It does not
// own a second MCP runtime or tool registry.
type Manager struct {
	repository *mcp.Repository
	stdio      *kernelmcp.CanonicalStdioRegistry
	remote     *kernelmcp.CanonicalRemoteRegistry
	secrets    auth.SecretStore
	oauth      OAuthTokens

	mu            sync.Mutex
	reconnecting  map[string]bool
	suppressed    map[string]bool
	readyHandlers []func(context.Context, string)
	root          context.Context
	cancel        context.CancelFunc
}

func NewManager(repository *mcp.Repository, stdio *kernelmcp.CanonicalStdioRegistry, remote *kernelmcp.CanonicalRemoteRegistry, secrets auth.SecretStore, oauth OAuthTokens) *Manager {
	root, cancel := context.WithCancel(context.Background())
	return &Manager{
		repository:   repository,
		stdio:        stdio,
		remote:       remote,
		secrets:      secrets,
		oauth:        oauth,
		reconnecting: map[string]bool{},
		suppressed:   map[string]bool{},
		root:         root,
		cancel:       cancel,
	}
}

func (m *Manager) RegisterReadyHandler(handler func(context.Context, string)) {
	if m == nil || handler == nil {
		return
	}
	m.mu.Lock()
	m.readyHandlers = append(m.readyHandlers, handler)
	m.mu.Unlock()
}

func (m *Manager) Restore(ctx context.Context) error {
	if m == nil || m.repository == nil {
		return fmt.Errorf("MCP repository unavailable")
	}
	servers, err := m.repository.ListEnabledServers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		serverID := server.ID
		go func() {
			if err := m.Connect(m.root, serverID); err != nil {
				m.scheduleReconnect(serverID)
			}
		}()
	}
	return nil
}

func (m *Manager) Connect(ctx context.Context, serverID string) error {
	if m == nil || m.repository == nil {
		return fmt.Errorf("MCP runtime unavailable")
	}
	m.mu.Lock()
	m.suppressed[serverID] = false
	m.mu.Unlock()
	server, err := m.repository.GetServer(ctx, serverID)
	if err != nil {
		return err
	}
	if _, ok := m.Connection(serverID); ok {
		return nil
	}
	_ = m.repository.SetServerStatus(context.Background(), server.ID, "connecting", "", "", nil)
	clientCapabilities := m.clientCapabilities(ctx, server.ID)

	var connection *client.Connection
	switch strings.ToLower(strings.TrimSpace(server.Transport)) {
	case "stdio":
		if m.stdio == nil {
			return m.fail(server.ID, fmt.Errorf("MCP stdio registry unavailable"))
		}
		var args []string
		if err := json.Unmarshal([]byte(server.ArgsJSON), &args); err != nil {
			return m.fail(server.ID, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: args"))
		}
		env, err := m.stdioEnvironment(ctx, server)
		if err != nil {
			return m.fail(server.ID, err)
		}
		conn, err := m.stdio.StartOrGet(ctx, kernelmcp.MCPStdioSpec{
			ServerID:     server.ID,
			Command:      server.Command,
			Args:         args,
			WorkDir:      server.WorkDir,
			Env:          env,
			Capabilities: clientCapabilities,
		})
		if err != nil {
			return m.fail(server.ID, err)
		}
		var ok bool
		connection, ok = conn.ClientConnection()
		if !ok {
			_ = m.stdio.Close(context.Background(), server.ID)
			return m.fail(server.ID, fmt.Errorf("MCP_SERVER_NOT_READY"))
		}
	case "streamable_http":
		if m.remote == nil {
			return m.fail(server.ID, fmt.Errorf("MCP remote registry unavailable"))
		}
		headers, err := m.httpHeaders(ctx, server)
		if err != nil {
			return m.fail(server.ID, err)
		}
		allowPrivate, _, _ := m.repository.ServerCapabilityEnabled(ctx, server.ID, "private_network")
		conn, err := m.remote.StartOrGet(ctx, kernelmcp.MCPRemoteSpec{
			ServerID:        server.ID,
			Endpoint:        server.Endpoint,
			Timeout:         30 * time.Second,
			MaxMessageBytes: 4 << 20,
			AllowLoopback:   true,
			AllowPrivate:    allowPrivate,
			AllowPublicHTTP: false,
			MaxRedirects:    3,
			StaticHeaders:   headers,
			Capabilities:    clientCapabilities,
		})
		if err != nil {
			return m.fail(server.ID, err)
		}
		var ok bool
		connection, ok = conn.ClientConnection()
		if !ok {
			_ = m.remote.Close(context.Background(), server.ID)
			return m.fail(server.ID, fmt.Errorf("MCP_SERVER_NOT_READY"))
		}
	default:
		return m.fail(server.ID, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: transport"))
	}

	initialized := connection.InitializeResult()
	serverInfo, _ := json.Marshal(initialized.ServerInfo)
	capabilities, _ := json.Marshal(initialized.Capabilities)
	persisted := &struct {
		ProtocolVersion  string
		ServerInfoJSON   string
		CapabilitiesJSON string
		Instructions     string
	}{
		ProtocolVersion:  initialized.ProtocolVersion,
		ServerInfoJSON:   string(serverInfo),
		CapabilitiesJSON: string(capabilities),
		Instructions:     initialized.Instructions,
	}
	if err := m.repository.SetServerStatus(ctx, server.ID, "ready", "", "", persisted); err != nil {
		_ = m.Disconnect(context.Background(), server.ID)
		return err
	}

	m.mu.Lock()
	handlers := append([]func(context.Context, string){}, m.readyHandlers...)
	m.mu.Unlock()
	for _, handler := range handlers {
		go handler(m.root, server.ID)
	}
	go func(id string, done <-chan struct{}) {
		select {
		case <-done:
			if !m.isSuppressed(id) {
				m.scheduleReconnect(id)
			}
		case <-m.root.Done():
		}
	}(server.ID, connection.Done())
	return nil
}

func (m *Manager) Disconnect(ctx context.Context, serverID string) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.reconnecting[serverID] = false
	m.suppressed[serverID] = true
	m.mu.Unlock()
	var firstErr error
	if m.stdio != nil {
		if err := m.stdio.Close(ctx, serverID); err != nil {
			firstErr = err
		}
	}
	if m.remote != nil {
		if err := m.remote.Close(ctx, serverID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if m.repository != nil {
		if err := m.repository.SetServerStatus(ctx, serverID, "disconnected", "", "", nil); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) Reconnect(ctx context.Context, serverID string) error {
	if err := m.Disconnect(ctx, serverID); err != nil {
		return err
	}
	return m.Connect(ctx, serverID)
}

func (m *Manager) Connection(serverID string) (*client.Connection, bool) {
	if m == nil {
		return nil, false
	}
	if m.stdio != nil {
		if conn, ok := m.stdio.Get(serverID); ok {
			if clientConn, ready := conn.ClientConnection(); ready {
				return clientConn, true
			}
		}
	}
	if m.remote != nil {
		if conn, ok := m.remote.Get(serverID); ok {
			if clientConn, ready := conn.ClientConnection(); ready {
				return clientConn, true
			}
		}
	}
	return nil, false
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
			m.scheduleReconnect(serverID)
		}
	}
	return result, err
}

func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.cancel()
	var firstErr error
	if m.stdio != nil {
		if err := m.stdio.CloseAll(ctx); err != nil {
			firstErr = err
		}
	}
	if m.remote != nil {
		if err := m.remote.CloseAll(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) fail(serverID string, err error) error {
	if err == nil {
		return nil
	}
	code := "MCP_TRANSPORT_START_FAILED"
	message := strings.TrimSpace(err.Error())
	if index := strings.Index(message, ":"); index > 0 && strings.HasPrefix(message[:index], "MCP_") {
		code = message[:index]
	}
	if m.repository != nil {
		_ = m.repository.SetServerStatus(context.Background(), serverID, "disconnected", code, message, nil)
	}
	return err
}

func (m *Manager) scheduleReconnect(serverID string) {
	if m == nil || m.repository == nil {
		return
	}
	m.mu.Lock()
	if m.reconnecting[serverID] || m.suppressed[serverID] {
		m.mu.Unlock()
		return
	}
	m.reconnecting[serverID] = true
	m.mu.Unlock()
	go func() {
		defer func() {
			m.mu.Lock()
			m.reconnecting[serverID] = false
			m.mu.Unlock()
		}()
		backoff := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}
		for _, delay := range backoff {
			if m.isSuppressed(serverID) {
				return
			}
			select {
			case <-time.After(delay):
			case <-m.root.Done():
				return
			}
			if m.isSuppressed(serverID) {
				return
			}
			server, err := m.repository.GetServer(m.root, serverID)
			if err != nil || server.Enabled != 1 {
				return
			}
			if err := m.Connect(m.root, serverID); err == nil {
				return
			}
		}
		_ = m.repository.SetServerStatus(context.Background(), serverID, "degraded", "MCP_RECONNECT_LIMIT_REACHED", "reconnect limit reached", nil)
	}()
}

func (m *Manager) isSuppressed(serverID string) bool {
	if m == nil {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.suppressed[serverID]
}

func (m *Manager) clientCapabilities(ctx context.Context, serverID string) protocol.ClientCapabilities {
	capabilities := protocol.ClientCapabilities{Experimental: map[string]any{}}
	if m == nil || m.repository == nil {
		return capabilities
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "roots"); err == nil && enabled {
		capabilities.Roots = map[string]any{"listChanged": true}
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "sampling"); err == nil && enabled {
		capabilities.Sampling = map[string]any{}
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "elicitation"); err == nil && enabled {
		capabilities.Elicitation = map[string]any{}
	}
	if enabled, _, err := m.repository.ServerCapabilityEnabled(ctx, serverID, "tasks"); err == nil && enabled {
		capabilities.Tasks = map[string]any{}
	}
	return capabilities
}

func (m *Manager) httpHeaders(ctx context.Context, server mcp.Server) (map[string]string, error) {
	headers := map[string]string{"Accept": "application/json, text/event-stream"}
	switch server.AuthType {
	case "", "none":
	case "oauth":
		if m.oauth == nil {
			return nil, fmt.Errorf("MCP_AUTH_REQUIRED")
		}
		token, err := m.oauth.AccessToken(ctx, server.ID)
		if err != nil {
			return nil, err
		}
		headers["Authorization"] = "Bearer " + token
	case "bearer_token":
		value, err := m.resolveCredential(ctx, server.ID, "bearer_token")
		if err != nil {
			return nil, err
		}
		headers["Authorization"] = "Bearer " + strings.TrimSpace(string(value))
	case "custom_headers":
		value, err := m.resolveCredential(ctx, server.ID, "custom_headers")
		if err != nil {
			return nil, err
		}
		configured := map[string]string{}
		if err := json.Unmarshal(value, &configured); err != nil {
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
	if parsed, err := url.Parse(server.Endpoint); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: endpoint")
	}
	return headers, nil
}

func (m *Manager) stdioEnvironment(ctx context.Context, server mcp.Server) (map[string]string, error) {
	if server.AuthType == "" || server.AuthType == "none" {
		return map[string]string{}, nil
	}
	if server.AuthType != "stdio_env" {
		return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: auth type")
	}
	value, err := m.resolveCredential(ctx, server.ID, "stdio_env")
	if err != nil {
		return nil, err
	}
	var environment map[string]string
	if err := json.Unmarshal(value, &environment); err != nil {
		return nil, fmt.Errorf("MCP_SERVER_CONFIGURATION_INVALID: stdio env")
	}
	return environment, nil
}

func (m *Manager) resolveCredential(ctx context.Context, serverID, credentialType string) ([]byte, error) {
	if m.repository == nil || m.secrets == nil {
		return nil, fmt.Errorf("MCP_AUTH_REQUIRED")
	}
	record, err := m.repository.CredentialReference(ctx, serverID, credentialType)
	if err != nil {
		return nil, fmt.Errorf("MCP_AUTH_REQUIRED")
	}
	return m.secrets.Get(ctx, record.SecretReference)
}
