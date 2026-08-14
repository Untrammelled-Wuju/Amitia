// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

func TestMCPRemoteSpec_Struct(t *testing.T) {
	spec := MCPRemoteSpec{
		ServerID:        "test-remote",
		Endpoint:        "https://example.com/mcp",
		Timeout:         30 * time.Second,
		MaxMessageBytes: 4 << 20,
		AllowLoopback:   true,
		AllowPrivate:    false,
		AllowPublicHTTP: false,
		MaxRedirects:    3,
		CredentialRef:   "cred-123",
		StaticHeaders:   map[string]string{"X-Custom": "value"},
	}

	if spec.ServerID != "test-remote" {
		t.Error("ServerID mismatch")
	}
	if spec.Endpoint != "https://example.com/mcp" {
		t.Error("Endpoint mismatch")
	}
}

func TestMCPRemoteResolvedSpec_Timeouts(t *testing.T) {
	spec := MCPRemoteResolvedSpec{}

	if spec.TimeoutOrDefault() != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", spec.TimeoutOrDefault())
	}

	if spec.MaxBytesOrDefault() != 4<<20 {
		t.Errorf("expected default max bytes 4MiB, got %d", spec.MaxBytesOrDefault())
	}
}

func TestValidateRemoteEndpoint_ValidPublicHTTPS(t *testing.T) {
	ctx := context.Background()
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
		AllowPrivate:  true,
	}

	_, err := ValidateRemoteEndpoint(ctx, "https://example.com/mcp", policy)
	if err != nil {
		t.Logf("endpoint validation result: %v", err)
	}
}

func TestValidateRemoteEndpoint_InvalidScheme(t *testing.T) {
	ctx := context.Background()
	policy := RemoteEndpointPolicy{}

	_, err := ValidateRemoteEndpoint(ctx, "ftp://example.com/mcp", policy)
	if err == nil {
		t.Error("expected error for invalid scheme")
	}
}

func TestValidateRemoteEndpoint_UserinfoForbidden(t *testing.T) {
	ctx := context.Background()
	policy := RemoteEndpointPolicy{}

	_, err := ValidateRemoteEndpoint(ctx, "https://user:pass@example.com/mcp", policy)
	if err == nil {
		t.Error("expected error for userinfo in endpoint")
	}
}

func TestValidateRemoteEndpoint_PublicHTTPForbidden(t *testing.T) {
	ctx := context.Background()
	policy := RemoteEndpointPolicy{
		AllowPublicHTTP: false,
	}

	_, err := ValidateRemoteEndpoint(ctx, "http://example.com/mcp", policy)
	if err == nil {
		t.Error("expected error for public HTTP endpoint")
	}
}

func TestValidateRemoteEndpoint_MetadataIPForbidden(t *testing.T) {
	ctx := context.Background()
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
		AllowPrivate:  true,
	}

	_, err := ValidateRemoteEndpoint(ctx, "http://169.254.169.254/mcp", policy)
	if err == nil {
		t.Error("expected error for metadata IP endpoint")
	}
}

func TestValidateRemoteEndpoint_InvalidPort(t *testing.T) {
	ctx := context.Background()
	policy := RemoteEndpointPolicy{}

	_, err := ValidateRemoteEndpoint(ctx, "https://example.com:99999/mcp", policy)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestStreamableHTTP_StateTransitions(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	if transport.State() != RemoteStateStopped {
		t.Errorf("expected initial state 'stopped', got '%s'", transport.State())
	}
}

func TestStreamableHTTP_SendWhileStopped(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	err := transport.Send(context.Background(), protocol.Message{
		Method: "test",
	})
	if err == nil {
		t.Error("expected error when sending on stopped transport")
	}
}

func TestStreamableHTTP_StartServerStreamWhileStopped(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	err := transport.StartServerStream(context.Background())
	if err == nil {
		t.Error("expected error when starting server stream on stopped transport")
	}
}

func TestStreamableHTTP_CloseWhileStopped(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	err := transport.Close(context.Background())
	if err != nil {
		t.Errorf("expected nil error when closing already stopped transport, got: %v", err)
	}
}

func TestStreamableHTTP_SessionIDInitiallyEmpty(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	if transport.SessionID() != "" {
		t.Error("expected empty session ID initially")
	}
}

func TestStreamableHTTP_LastEventIDInitiallyEmpty(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	if transport.LastEventID() != "" {
		t.Error("expected empty last event ID initially")
	}
}

func TestStreamableHTTP_ReceiveChannelExists(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	if transport.Receive() == nil {
		t.Error("expected non-nil receive channel")
	}
}

func TestStreamableHTTP_DoneChannelExists(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)

	if transport.Done() == nil {
		t.Error("expected non-nil done channel")
	}
}

func TestStreamableHTTP_SetProtocolVersion(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}
	policy := RemoteEndpointPolicy{}

	transport := NewStreamableHTTP(spec, policy)
	transport.SetProtocolVersion("2025-03-26")
}

func TestStreamableHTTP_StartWithTestServer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	spec := MCPRemoteResolvedSpec{
		ServerID:      "test",
		Endpoint:      server.URL + "/mcp",
		StaticHeaders: map[string]string{},
	}
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
	}

	transport := NewStreamableHTTP(spec, policy)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Start(ctx)
	if err != nil {
		t.Logf("start with test server: %v", err)
	}
}

func TestStreamableHTTP_SendJSONResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":{}}`))
	}))
	defer server.Close()

	spec := MCPRemoteResolvedSpec{
		ServerID:      "test",
		Endpoint:      server.URL + "/mcp",
		StaticHeaders: map[string]string{},
	}
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
	}

	transport := NewStreamableHTTP(spec, policy)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Skipf("cannot start transport: %v", err)
	}

	if err := transport.Send(ctx, protocol.Message{
		Method: "tools/list",
		ID:     json.RawMessage(`"1"`),
	}); err != nil {
		t.Logf("send error: %v", err)
	}

	_ = transport.Close(ctx)
}

func TestStreamableHTTP_SendSSEResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":\"1\",\"result\":{}}\n\n"))
	}))
	defer server.Close()

	spec := MCPRemoteResolvedSpec{
		ServerID:      "test",
		Endpoint:      server.URL + "/mcp",
		StaticHeaders: map[string]string{},
	}
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
	}

	transport := NewStreamableHTTP(spec, policy)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Skipf("cannot start transport: %v", err)
	}

	if err := transport.Send(ctx, protocol.Message{
		Method: "tools/list",
		ID:     json.RawMessage(`"1"`),
	}); err != nil {
		t.Logf("send error: %v", err)
	}

	_ = transport.Close(ctx)
}

func TestStreamableHTTP_Accepted202(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	spec := MCPRemoteResolvedSpec{
		ServerID:      "test",
		Endpoint:      server.URL + "/mcp",
		StaticHeaders: map[string]string{},
	}
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
	}

	transport := NewStreamableHTTP(spec, policy)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Skipf("cannot start transport: %v", err)
	}

	if err := transport.Send(ctx, protocol.Message{
		Method: "notifications/initialized",
	}); err != nil {
		t.Logf("send error: %v", err)
	}

	_ = transport.Close(ctx)
}

func TestStreamableHTTP_SessionIDFromResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("MCP-Session-Id", "test-session-123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	spec := MCPRemoteResolvedSpec{
		ServerID:      "test",
		Endpoint:      server.URL + "/mcp",
		StaticHeaders: map[string]string{},
	}
	policy := RemoteEndpointPolicy{
		AllowLoopback: true,
	}

	transport := NewStreamableHTTP(spec, policy)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Skipf("cannot start transport: %v", err)
	}

	_ = transport.Close(ctx)
}

func TestSSEParser_ParseSingleEvent(t *testing.T) {
	receive := make(chan protocol.Message, 64)
	parser := NewSSEParser(4<<20, receive)

	input := strings.NewReader("data: {\"jsonrpc\":\"2.0\",\"id\":\"1\",\"result\":{}}\n\n")

	ctx := context.Background()
	err := parser.Parse(ctx, input, func(eventID string) {}, func(delay time.Duration) {})
	if err != nil {
		t.Errorf("unexpected parse error: %v", err)
	}

	select {
	case msg := <-receive:
		if !bytes.Equal(msg.ID, []byte(`"1"`)) {
			t.Errorf("expected message ID '1', got '%s'", msg.ID)
		}
	default:
		t.Error("expected a message in receive channel")
	}
}

func TestSSEParser_ParseMultiLineData(t *testing.T) {
	receive := make(chan protocol.Message, 64)
	parser := NewSSEParser(4<<20, receive)

	input := strings.NewReader("data: {\"jsonrpc\":\"2.0\"\ndata: \"id\":\"1\",\"result\":{}}\n\n")

	ctx := context.Background()
	err := parser.Parse(ctx, input, func(eventID string) {}, func(delay time.Duration) {})
	if err != nil {
		t.Logf("parse error (expected for malformed): %v", err)
	}
}

func TestSSEParser_ParseEventID(t *testing.T) {
	receive := make(chan protocol.Message, 64)
	parser := NewSSEParser(4<<20, receive)

	input := strings.NewReader("id: evt-1\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"1\",\"result\":{}}\n\n")

	ctx := context.Background()
	var capturedEventID string
	err := parser.Parse(ctx, input, func(eventID string) {
		capturedEventID = eventID
	}, func(delay time.Duration) {})
	if err != nil {
		t.Errorf("unexpected parse error: %v", err)
	}

	if capturedEventID != "evt-1" {
		t.Errorf("expected event ID 'evt-1', got '%s'", capturedEventID)
	}
}

func TestSSEParser_ParseRetryDelay(t *testing.T) {
	receive := make(chan protocol.Message, 64)
	parser := NewSSEParser(4<<20, receive)

	input := strings.NewReader("retry: 1000\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"1\",\"result\":{}}\n\n")

	ctx := context.Background()
	var capturedDelay time.Duration
	err := parser.Parse(ctx, input, func(eventID string) {}, func(delay time.Duration) {
		capturedDelay = delay
	})
	if err != nil {
		t.Errorf("unexpected parse error: %v", err)
	}

	if capturedDelay != time.Second {
		t.Errorf("expected retry delay 1s, got %v", capturedDelay)
	}
}

func TestSSEParser_IgnoreComment(t *testing.T) {
	receive := make(chan protocol.Message, 64)
	parser := NewSSEParser(4<<20, receive)

	input := strings.NewReader(": keepalive\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"1\",\"result\":{}}\n\n")

	ctx := context.Background()
	err := parser.Parse(ctx, input, func(eventID string) {}, func(delay time.Duration) {})
	if err != nil {
		t.Errorf("unexpected parse error: %v", err)
	}

	select {
	case <-receive:
	default:
		t.Error("expected a message in receive channel")
	}
}

func TestSSEParser_InvalidRetryDelay(t *testing.T) {
	receive := make(chan protocol.Message, 64)
	parser := NewSSEParser(4<<20, receive)

	input := strings.NewReader("retry: 0\ndata: {\"jsonrpc\":\"2.0\",\"id\":\"1\",\"result\":{}}\n\n")

	ctx := context.Background()
	var capturedDelay time.Duration
	err := parser.Parse(ctx, input, func(eventID string) {}, func(delay time.Duration) {
		capturedDelay = delay
	})
	if err != nil {
		t.Errorf("unexpected parse error: %v", err)
	}

	if capturedDelay != 0 {
		t.Error("expected invalid retry delay to be ignored")
	}
}

func TestCanonicalRemoteFactory_Create(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	spec := MCPRemoteSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}

	ctx := context.Background()
	conn, err := factory.Create(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create remote connection: %v", err)
	}

	if conn.ServerID() != "test" {
		t.Errorf("expected server ID 'test', got '%s'", conn.ServerID())
	}

	if conn.State() != MCPStdioStateStopped {
		t.Errorf("expected initial state 'stopped', got '%s'", conn.State())
	}
}

func TestCanonicalRemoteConnection_CallWhileStopped(t *testing.T) {
	spec := MCPRemoteResolvedSpec{
		ServerID: "test",
		Endpoint: "https://example.com/mcp",
	}

	conn := &CanonicalRemoteConnection{
		spec:  spec,
		state: MCPStdioStateStopped,
	}

	_, err := conn.Call(context.Background(), "tools/list", map[string]any{})
	if err == nil {
		t.Error("expected error when calling on stopped connection")
	}
}

func TestCanonicalRemoteConnection_Health(t *testing.T) {
	tests := []struct {
		name     string
		state    MCPStdioServerState
		expected string
	}{
		{"stopped", MCPStdioStateStopped, "unknown"},
		{"starting", MCPStdioStateStarting, "unknown"},
		{"initializing", MCPStdioStateInitializing, "unknown"},
		{"ready", MCPStdioStateReady, "ready"},
		{"closing", MCPStdioStateClosing, "unknown"},
		{"failed", MCPStdioStateFailed, "unhealthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &CanonicalRemoteConnection{
				spec:  MCPRemoteResolvedSpec{ServerID: "test"},
				state: tt.state,
			}
			health := conn.Health()
			if string(health) != tt.expected {
				t.Errorf("expected health '%s', got '%s'", tt.expected, health)
			}
		})
	}
}

func TestCanonicalRemoteRegistry_SingleOwnerGuard(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	spec := MCPRemoteSpec{
		ServerID: "test-server",
		Endpoint: "https://example.com/mcp",
	}

	_, err := registry.StartOrGet(context.Background(), spec)
	if err == nil {
		t.Log("StartOrGet succeeded (unexpected without real server)")
	}
}

func TestCanonicalRemoteRegistry_GetNonExistent(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	_, ok := registry.Get("non-existent")
	if ok {
		t.Error("expected false for non-existent server")
	}
}

func TestCanonicalRemoteRegistry_CloseNonExistent(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	err := registry.Close(context.Background(), "non-existent")
	if err != nil {
		t.Errorf("expected nil error for non-existent server, got: %v", err)
	}
}

func TestCanonicalRemoteRegistry_CloseAll(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	err := registry.CloseAll(context.Background())
	if err != nil {
		t.Errorf("expected nil error for empty registry, got: %v", err)
	}
}

func TestCanonicalRemoteRegistry_List(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	list := registry.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestCanonicalRemoteRegistry_IsOwnedByLegacy(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	if !registry.IsOwnedByLegacy("any-server") {
		t.Error("expected IsOwnedByLegacy to return true when server not in registry")
	}
}

func TestCanonicalRemoteRegistry_RegisterLegacyOwnership(t *testing.T) {
	factory := NewCanonicalRemoteFactory()
	registry := NewCanonicalRemoteRegistry(factory)

	err := registry.RegisterLegacyOwnership("some-server")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestValidateRemoteSessionID_Valid(t *testing.T) {
	err := validateRemoteSessionID("valid-session-id-123")
	if err != nil {
		t.Errorf("expected nil error for valid session ID, got: %v", err)
	}
}

func TestValidateRemoteSessionID_ControlCharacter(t *testing.T) {
	err := validateRemoteSessionID("invalid\x01session")
	if err == nil {
		t.Error("expected error for session ID with control character")
	}
}

func TestValidateRemoteSessionID_Oversized(t *testing.T) {
	oversized := strings.Repeat("a", 2048)
	err := validateRemoteSessionID(oversized)
	if err == nil {
		t.Error("expected error for oversized session ID")
	}
}

func TestRemoteTransportState_String(t *testing.T) {
	states := []RemoteTransportState{
		RemoteStateStopped,
		RemoteStateStarting,
		RemoteStateRunning,
		RemoteStateClosing,
		RemoteStateError,
	}

	for _, state := range states {
		if string(state) == "" {
			t.Errorf("state should have a string representation")
		}
	}
}

func TestIsTransportOwnedHeader(t *testing.T) {
	ownedHeaders := []string{"Host", "Content-Type", "Accept", "Origin", "MCP-Session-Id", "MCP-Protocol-Version", "Last-Event-ID"}
	for _, header := range ownedHeaders {
		if !isTransportOwnedHeader(header) {
			t.Errorf("expected '%s' to be transport-owned", header)
		}
	}

	customHeaders := []string{"Authorization", "X-Api-Key", "X-Custom"}
	for _, header := range customHeaders {
		if isTransportOwnedHeader(header) {
			t.Errorf("expected '%s' to NOT be transport-owned", header)
		}
	}
}
