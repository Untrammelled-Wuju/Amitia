package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	mcp "github.com/u-ai/backend/internal/extension/mcpruntime"
	"github.com/u-ai/backend/internal/extension/mcpruntime/client"
	"github.com/u-ai/backend/internal/extension/mcpruntime/protocol"
	mcpmigration "github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
	"gorm.io/gorm"
)

func TestContributionBindingLifecycleAndRecovery(t *testing.T) {
	runtime, dataDir, db, repository, registry, closeRuntime := newTestRuntime(t)
	defer closeRuntime()

	binding := kernel.MCPBindingDescriptor{
		ExtensionID:       "example.test/mcp-package",
		ModuleID:          "main",
		ContributionID:    "echo-server",
		PackageGeneration: 7,
		ServerName:        "echo-server",
		DisplayName:       "Echo Server",
		Transport:         "stdio",
		Command:           os.Args[0],
		Args:              []string{"-test.run=^TestMCPHelperProcess$"},
		Env:               map[string]string{"GO_WANT_MCP_HELPER": "1"},
		RuntimePlacement:  "local",
	}

	serverID, err := runtime.PutContributionBinding(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	server, err := repository.GetServer(context.Background(), serverID)
	if err != nil {
		t.Fatal(err)
	}
	if server.OwnerKind != "package" || server.OwnerExtensionID != binding.ExtensionID || server.OwnerContributionID != binding.ContributionID {
		t.Fatalf("unexpected package ownership: %#v", server)
	}
	if server.Enabled != 0 || server.PackageGeneration != 7 {
		t.Fatalf("unexpected installed state: enabled=%d generation=%d", server.Enabled, server.PackageGeneration)
	}
	if _, connected := runtime.Connections.Connection(serverID); connected {
		t.Fatal("install must not start the MCP runtime")
	}

	if _, err := runtime.ActivateContributionBinding(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	server, err = repository.GetServer(context.Background(), serverID)
	if err != nil {
		t.Fatal(err)
	}
	if server.Enabled != 1 || server.DesiredEnabled != 1 || server.Status != "ready" {
		t.Fatalf("unexpected active state: enabled=%d desired=%d status=%s", server.Enabled, server.DesiredEnabled, server.Status)
	}
	if server.CapabilitiesJSON == "" || server.CapabilitiesJSON == "{}" {
		t.Fatalf("MCP capabilities were not persisted: %s", server.CapabilitiesJSON)
	}
	tools, err := repository.ListTools(context.Background(), serverID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("discovered MCP tools = %d, capabilities=%s", len(tools), server.CapabilitiesJSON)
	}
	if err := repository.SetToolEnabled(context.Background(), tools[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tools.RegisterServer(context.Background(), serverID); err != nil {
		t.Fatal(err)
	}
	assertMCPToolRegistered(t, registry, 1)
	raw, err := runtime.Connections.Call(context.Background(), serverID, "tools/call", map[string]any{
		"name":      "echo",
		"arguments": map[string]any{"value": "production"},
	}, client.CallOptions{})
	if err != nil || string(raw) == "" {
		t.Fatalf("MCP tool call failed: %s err=%v", raw, err)
	}

	if err := runtime.DeactivateContributionBinding(context.Background(), binding.ExtensionID, binding.ContributionID); err != nil {
		t.Fatal(err)
	}
	assertMCPToolRegistered(t, registry, 0)
	if _, connected := runtime.Connections.Connection(serverID); connected {
		t.Fatal("deactivate must stop the MCP runtime")
	}
	server, err = repository.GetServer(context.Background(), serverID)
	if err != nil {
		t.Fatal(err)
	}
	if server.Enabled != 0 || server.DesiredEnabled != 0 {
		t.Fatalf("deactivate state mismatch: enabled=%d desired=%d", server.Enabled, server.DesiredEnabled)
	}

	if _, err := runtime.ActivateContributionBinding(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Connections.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	restartedRegistry := capability.NewToolRegistry()
	restarted, err := buildTestRuntime(t, dataDir, db, repository, restartedRegistry)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Connections.Close(context.Background())
	waitForMCPConnection(t, repository, restarted, serverID)
	waitForMCPTools(t, restartedRegistry, 1)
	assertMCPToolRegistered(t, restartedRegistry, 1)

	if err := restarted.RemoveContributionBinding(context.Background(), binding.ExtensionID, binding.ContributionID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetServer(context.Background(), serverID); err == nil {
		t.Fatal("uninstall must delete the MCP binding")
	}
	assertMCPToolRegistered(t, restartedRegistry, 0)
}

func TestContributionBindingPlacementIsEnforced(t *testing.T) {
	tests := []struct {
		name      string
		profile   runtimeprofile.Profile
		placement string
		hosted    bool
	}{
		{"local hosts device binding", runtimeprofile.ProfileLocal, "device", true},
		{"cloud hosts cloud binding", runtimeprofile.ProfileCloudCore, "cloud", true},
		{"cloud rejects device binding", runtimeprofile.ProfileCloudCore, "device", false},
		{"device hosts device binding", runtimeprofile.ProfileDeviceAgent, "device", true},
		{"device rejects cloud binding", runtimeprofile.ProfileDeviceAgent, "cloud", false},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := capability.NewToolRegistry()
			db := newMCPTestDB(t)
			repository := mcp.NewRepository(db)
			resolver, err := commandenv.NewResolver(commandenv.ResolveContext{})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := NewRuntime(
				db,
				repository,
				kernelmcp.NewCanonicalStdioRegistry(kernelmcp.NewCanonicalStdioFactory(resolver)),
				kernelmcp.NewCanonicalRemoteRegistry(kernelmcp.NewCanonicalRemoteFactory()),
				resolver,
				kernel.NewToolFacade(registry, nil),
				nil,
				t.TempDir(),
				test.profile,
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Connections.Close(context.Background())
			binding := kernel.MCPBindingDescriptor{
				ExtensionID:      fmt.Sprintf("example.test/package-%d", index),
				ModuleID:         "main",
				ContributionID:   "server",
				ServerName:       "server",
				Transport:        "stdio",
				Command:          os.Args[0],
				Args:             []string{"-test.run=^TestMCPHelperProcess$"},
				Env:              map[string]string{"GO_WANT_MCP_HELPER": "1"},
				RuntimePlacement: test.placement,
			}
			serverID, err := runtime.ActivateContributionBinding(context.Background(), binding)
			if err != nil {
				t.Fatal(err)
			}
			_, connected := runtime.Connections.Connection(serverID)
			if connected != test.hosted {
				t.Fatalf("connected=%v, want %v", connected, test.hosted)
			}
			assertMCPToolRegistered(t, registry, 0)
		})
	}
}

func TestRemoteContributionBindingLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(serveTestMCPHTTP))
	defer server.Close()

	runtime, _, _, repository, registry, closeRuntime := newTestRuntime(t)
	defer closeRuntime()
	binding := kernel.MCPBindingDescriptor{
		ExtensionID:      "example.test/remote-package",
		ModuleID:         "main",
		ContributionID:   "remote-server",
		ServerName:       "remote-server",
		Transport:        "streamable_http",
		Endpoint:         server.URL,
		RuntimePlacement: "local",
	}
	serverID, err := runtime.ActivateContributionBinding(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := repository.ListTools(context.Background(), serverID, false)
	if err != nil || len(tools) != 1 {
		t.Fatalf("remote tools=%d err=%v", len(tools), err)
	}
	if err := repository.SetToolEnabled(context.Background(), tools[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tools.RegisterServer(context.Background(), serverID); err != nil {
		t.Fatal(err)
	}
	assertMCPToolRegistered(t, registry, 1)
	raw, err := runtime.Connections.Call(context.Background(), serverID, "tools/call", map[string]any{
		"name":      "echo",
		"arguments": map[string]any{"value": "remote"},
	}, client.CallOptions{})
	if err != nil || string(raw) == "" {
		t.Fatalf("remote MCP tool call failed: %s err=%v", raw, err)
	}
	if err := runtime.DeactivateContributionBinding(context.Background(), binding.ExtensionID, binding.ContributionID); err != nil {
		t.Fatal(err)
	}
	assertMCPToolRegistered(t, registry, 0)
}

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request protocol.Message
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			return
		}
		if request.Method == "notifications/initialized" {
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": protocol.LatestProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "fixture", "version": "1.0.0"},
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "echo",
				"title":       "Echo",
				"description": "Echo a value",
				"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}},
			}}}
		case "tools/call":
			result = map[string]any{
				"content":           []map[string]any{{"type": "text", "text": "production"}},
				"structuredContent": map[string]any{"ok": true},
			}
		case "ping":
			result = map[string]any{}
		default:
			response := protocol.Message{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error:   &protocol.RPCError{Code: protocol.ErrorMethodNotFound, Message: "method not found"},
			}
			_ = encoder.Encode(response)
			continue
		}
		raw, _ := json.Marshal(result)
		response := protocol.Message{JSONRPC: "2.0", ID: request.ID, Result: raw}
		if err := encoder.Encode(response); err != nil {
			return
		}
	}
}

func serveTestMCPHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var message protocol.Message
	if json.NewDecoder(request.Body).Decode(&message) != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if message.Method == "notifications/initialized" {
		writer.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch message.Method {
	case "initialize":
		writer.Header().Set("MCP-Session-Id", "test-session")
		result = map[string]any{
			"protocolVersion": protocol.LatestProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "remote-fixture", "version": "1.0.0"},
		}
	case "tools/list":
		result = map[string]any{"tools": []map[string]any{{
			"name":        "echo",
			"title":       "Echo",
			"description": "Echo a value",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}},
		}}}
	case "tools/call":
		result = map[string]any{
			"content":           []map[string]any{{"type": "text", "text": "remote"}},
			"structuredContent": map[string]any{"ok": true},
		}
	default:
		response := protocol.Message{JSONRPC: "2.0", ID: message.ID, Error: &protocol.RPCError{Code: protocol.ErrorMethodNotFound, Message: "method not found"}}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(response)
		return
	}
	response, _ := protocol.Response(message.ID, result)
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

func newTestRuntime(t *testing.T) (*Runtime, string, *gorm.DB, *mcp.Repository, *capability.ToolRegistry, func()) {
	t.Helper()
	dataDir := t.TempDir()
	db := newMCPTestDB(t)
	repository := mcp.NewRepository(db)
	registry := capability.NewToolRegistry()
	runtime, err := buildTestRuntime(t, dataDir, db, repository, registry)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, dataDir, db, repository, registry, func() {
		_ = runtime.Connections.Close(context.Background())
	}
}

func buildTestRuntime(t *testing.T, dataDir string, db *gorm.DB, repository *mcp.Repository, registry *capability.ToolRegistry) (*Runtime, error) {
	t.Helper()
	resolver, err := commandenv.NewResolver(commandenv.ResolveContext{})
	if err != nil {
		return nil, err
	}
	runtime, err := NewRuntime(
		db,
		repository,
		kernelmcp.NewCanonicalStdioRegistry(kernelmcp.NewCanonicalStdioFactory(resolver)),
		kernelmcp.NewCanonicalRemoteRegistry(kernelmcp.NewCanonicalRemoteFactory()),
		resolver,
		kernel.NewToolFacade(registry, nil),
		nil,
		dataDir,
		runtimeprofile.ProfileLocal,
	)
	return runtime, err
}

func newMCPTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:mcp-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := mcpmigration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]mcpmigration.Migration{mcpmigration.MCPClientMigration(), mcpmigration.MCPOwnedBindingsMigration()}); err != nil {
		t.Fatal(err)
	}
	return db
}

func waitForMCPTools(t *testing.T, registry *capability.ToolRegistry, expected int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if registry.CountBySource(capability.ToolSourceMCP) == expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertMCPToolRegistered(t, registry, expected)
}

func waitForMCPConnection(t *testing.T, repository *mcp.Repository, runtime *Runtime, serverID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		server, err := repository.GetServer(context.Background(), serverID)
		if err == nil {
			if _, connected := runtime.Connections.Connection(serverID); connected && server.Status == "ready" {
				return
			}
			if server.Status == "disconnected" && server.LastErrorCode != "" {
				t.Fatalf("MCP recovery failed: %s %s", server.LastErrorCode, server.LastErrorMessage)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	server, _ := repository.GetServer(context.Background(), serverID)
	t.Fatalf("MCP recovery timed out: status=%s error=%s %s", server.Status, server.LastErrorCode, server.LastErrorMessage)
}

func assertMCPToolRegistered(t *testing.T, registry *capability.ToolRegistry, expected int) {
	t.Helper()
	if actual := registry.CountBySource(capability.ToolSourceMCP); actual != expected {
		t.Fatalf("MCP tool count = %d, want %d", actual, expected)
	}
}
