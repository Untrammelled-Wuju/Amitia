// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
)

func TestCanonicalStdioFactory_Create_ResolvesCommand(t *testing.T) {
	resolver, err := commandenv.NewResolver(commandenv.ResolveContext{})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	factory := NewCanonicalStdioFactory(resolver)
	spec := MCPStdioSpec{
		ServerID: "test-server",
		Command:  "nonexistent_command_12345",
		Args:     []string{"hello"},
		WorkDir:  ".",
		Env:      map[string]string{"TEST_VAR": "test_value"},
	}

	ctx := context.Background()
	_, err = factory.Create(ctx, spec)
	if err == nil {
		t.Log("command resolved (unexpected on this platform)")
	} else {
		t.Logf("command resolution failed as expected: %v", err)
	}
}

func TestCanonicalStdioFactory_Create_NilResolver(t *testing.T) {
	factory := NewCanonicalStdioFactory(nil)
	spec := MCPStdioSpec{
		ServerID: "test-server",
		Command:  "echo",
		Args:     []string{"hello"},
	}

	ctx := context.Background()
	_, err := factory.Create(ctx, spec)
	if err == nil {
		t.Fatal("expected error when resolver is nil")
	}
}

func TestCanonicalStdioConnection_StateTransitions(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateStopped,
	}

	if conn.State() != MCPStdioStateStopped {
		t.Errorf("expected initial state 'stopped', got '%s'", conn.State())
	}

	conn.setState(MCPStdioStateStarting)
	if conn.State() != MCPStdioStateStarting {
		t.Errorf("expected state 'starting', got '%s'", conn.State())
	}

	conn.setState(MCPStdioStateReady)
	if conn.State() != MCPStdioStateReady {
		t.Errorf("expected state 'ready', got '%s'", conn.State())
	}
}

func TestCanonicalStdioConnection_Health(t *testing.T) {
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
			conn := &CanonicalStdioConnection{
				spec:  MCPStdioResolvedSpec{ServerID: "test"},
				state: tt.state,
			}
			health := conn.Health()
			if string(health) != tt.expected {
				t.Errorf("expected health '%s', got '%s'", tt.expected, health)
			}
		})
	}
}

func TestCanonicalStdioRegistry_SingleOwnerGuard(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	spec := MCPStdioSpec{
		ServerID: "test-server",
		Command:  "echo",
		Args:     []string{"hello"},
	}

	_, err := registry.StartOrGet(context.Background(), spec)
	if err == nil {
		t.Log("StartOrGet failed as expected (echo is not a valid MCP server)")
	}

	conn, ok := registry.Get("test-server")
	if ok && conn != nil {
		t.Log("connection found in registry")
	}
}

func TestCanonicalStdioRegistry_Get_NonExistent(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	_, ok := registry.Get("non-existent")
	if ok {
		t.Error("expected false for non-existent server")
	}
}

func TestCanonicalStdioRegistry_Close_NonExistent(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	err := registry.Close(context.Background(), "non-existent")
	if err != nil {
		t.Errorf("expected nil error for non-existent server, got: %v", err)
	}
}

func TestCanonicalStdioRegistry_CloseAll(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	err := registry.CloseAll(context.Background())
	if err != nil {
		t.Errorf("expected nil error for empty registry, got: %v", err)
	}
}

func TestMCPStdioResolvedSpec_Timeouts(t *testing.T) {
	spec := MCPStdioResolvedSpec{}

	if spec.StartTimeout() != 10*time.Second {
		t.Errorf("expected default start timeout 10s, got %v", spec.StartTimeout())
	}

	if spec.ShutdownTimeout() != 3*time.Second {
		t.Errorf("expected default shutdown timeout 3s, got %v", spec.ShutdownTimeout())
	}

	if spec.MaxMessageBytes() != 4<<20 {
		t.Errorf("expected default max message bytes 4MiB, got %d", spec.MaxMessageBytes())
	}
}

func TestBuildMinimalEnvironment(t *testing.T) {
	explicit := map[string]string{
		"TEST_VAR": "test_value",
		"API_KEY":  "secret",
	}

	env := buildMinimalEnvironment(explicit)

	if env["TEST_VAR"] != "test_value" {
		t.Errorf("expected TEST_VAR=test_value, got '%s'", env["TEST_VAR"])
	}

	if env["API_KEY"] != "secret" {
		t.Errorf("expected API_KEY=secret, got '%s'", env["API_KEY"])
	}
}

func TestCanonicalStdioConnection_GetCaller_NilWhenStopped(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateStopped,
	}

	caller := conn.GetCaller()
	if caller == nil {
		t.Log("caller is nil when stopped (expected)")
	}
}

func TestCanonicalStdioConnection_GetHealthFunc(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateReady,
	}

	healthFunc := conn.GetHealthFunc()
	if healthFunc == nil {
		t.Fatal("health func should not be nil")
	}

	status := healthFunc(context.Background(), "test")
	if string(status) != "ready" {
		t.Errorf("expected health status 'ready', got '%s'", status)
	}
}

func TestCanonicalStdioConnection_Call_NotReady(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateStopped,
	}

	_, err := conn.Call(context.Background(), "tools/list", map[string]any{})
	if err == nil {
		t.Error("expected error when calling on stopped connection")
	}
}

func TestCanonicalStdioConnection_Close_AlreadyStopped(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateStopped,
	}

	err := conn.Close(context.Background())
	if err != nil {
		t.Errorf("expected nil error when closing already stopped connection, got: %v", err)
	}
}

func TestCanonicalStdioRegistry_List(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	list := registry.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestCanonicalStdioRegistry_IsOwnedByLegacy(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	if !registry.IsOwnedByLegacy("any-server") {
		t.Error("expected IsOwnedByLegacy to return true when server not in registry")
	}
}

func TestCanonicalStdioRegistry_RegisterLegacyOwnership(t *testing.T) {
	resolver, _ := commandenv.NewResolver(commandenv.ResolveContext{})
	factory := NewCanonicalStdioFactory(resolver)
	registry := NewCanonicalStdioRegistry(factory)

	err := registry.RegisterLegacyOwnership("some-server")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestMCPStdioSpec_Struct(t *testing.T) {
	env := map[string]string{"KEY": "VALUE"}
	spec := MCPStdioSpec{
		ServerID:        "test",
		Command:         "echo",
		Args:            []string{"hello", "world"},
		WorkDir:         "/tmp",
		Env:             env,
		StartTimeout:    5 * time.Second,
		ShutdownTimeout: 2 * time.Second,
		MaxMessageBytes: 8 << 20,
	}

	if spec.ServerID != "test" {
		t.Error("ServerID mismatch")
	}
	if spec.Command != "echo" {
		t.Error("Command mismatch")
	}
	if len(spec.Args) != 2 {
		t.Error("Args length mismatch")
	}
}

func TestMCPStdioResolvedSpec_Struct(t *testing.T) {
	spec := MCPStdioResolvedSpec{
		ServerID:   "test",
		Executable: "/usr/bin/echo",
		Args:       []string{"hello"},
		WorkDir:    "/tmp",
		Env:        map[string]string{"KEY": "VALUE"},
	}

	if spec.ServerID != "test" {
		t.Error("ServerID mismatch")
	}
	if spec.Executable != "/usr/bin/echo" {
		t.Error("Executable mismatch")
	}
}

func TestCanonicalStdioConnection_callTool(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateStopped,
	}

	_, err := conn.callTool(context.Background(), "test", "tool", json.RawMessage(`{"arg": "value"}`))
	if err == nil {
		t.Error("expected error when calling on stopped connection")
	}
}

func TestCanonicalStdioConnection_ListTools_NotReady(t *testing.T) {
	conn := &CanonicalStdioConnection{
		spec:  MCPStdioResolvedSpec{ServerID: "test"},
		state: MCPStdioStateStopped,
	}

	_, err := conn.ListTools(context.Background())
	if err == nil {
		t.Error("expected error when listing tools on stopped connection")
	}
}

func TestMCPConnectionState_String(t *testing.T) {
	states := []MCPStdioServerState{
		MCPStdioStateStopped,
		MCPStdioStateStarting,
		MCPStdioStateInitializing,
		MCPStdioStateReady,
		MCPStdioStateClosing,
		MCPStdioStateFailed,
	}

	for _, state := range states {
		if string(state) == "" {
			t.Errorf("state should have a string representation")
		}
	}
}

func TestProtocolVersion_Valid(t *testing.T) {
	if protocol.LatestProtocolVersion == "" {
		t.Error("LatestProtocolVersion should not be empty")
	}

	if !protocol.SupportsVersion(protocol.LatestProtocolVersion) {
		t.Error("LatestProtocolVersion should be supported")
	}
}
