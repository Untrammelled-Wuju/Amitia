package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/auth"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func mcpBaselineDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Server{}, &ServerScopeBinding{}, &ServerCredential{}, &ServerCapability{}, &ToolDefinition{}, &ResourceDefinition{}, &ResourceTemplate{}, &PromptDefinition{}, &DependencyLink{}, &Task{}, &OAuthSession{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLegacy_MCP_ServerCRUD(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	srv, err := repo.CreateServer(ctx, ServerInput{
		Name:      "test-server",
		Transport: "streamable_http",
		Endpoint:  "https://example.com/mcp",
		AuthType:  "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.ID == "" || srv.Transport != "streamable_http" {
		t.Fatalf("invalid created server: %+v", srv)
	}

	created, err := repo.GetServer(ctx, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-server" {
		t.Fatalf("name mismatch: %s", created.Name)
	}

	updated, err := repo.UpdateServer(ctx, srv.ID, ServerInput{
		Name:        "test-server",
		Transport:   "streamable_http",
		Endpoint:    "https://example.com/mcp",
		AuthType:    "none",
		DisplayName: "Updated",
		Enabled:     false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "Updated" || updated.Enabled != 0 {
		t.Fatalf("update not persisted: %+v", updated)
	}

	if err := repo.DeleteServer(ctx, srv.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetServer(ctx, srv.ID); err == nil {
		t.Fatal("server was not deleted")
	}
}

func TestLegacy_MCP_DuplicateIdentityRejected(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	_, err := repo.CreateServer(ctx, ServerInput{Name: "a", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateServer(ctx, ServerInput{Name: "b", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err == nil {
		t.Fatal("expected duplicate identity rejection")
	}
}

func TestLegacy_MCP_DuplicateNameAllowed(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	_, err := repo.CreateServer(ctx, ServerInput{Name: "same-name", Transport: "streamable_http", Endpoint: "https://a.example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateServer(ctx, ServerInput{Name: "same-name", Transport: "streamable_http", Endpoint: "https://b.example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatalf("same name with different identity should be allowed: %v", err)
	}
}

func TestLegacy_MCP_TransportValidation(t *testing.T) {
	cases := []struct {
		name   string
		input  ServerInput
		reject bool
	}{
		{"valid http", ServerInput{Transport: "streamable_http", Endpoint: "https://example.com/mcp"}, false},
		{"valid stdio", ServerInput{Transport: "stdio", Command: "node", Args: []string{"server.js"}}, false},
		{"invalid transport", ServerInput{Transport: "unsupported", Endpoint: "http://example.com"}, true},
		{"ftp endpoint", ServerInput{Transport: "streamable_http", Endpoint: "ftp://example.com/mcp"}, true},
		{"http credentials", ServerInput{Transport: "streamable_http", Endpoint: "https://user:pass@example.com/mcp"}, true},
		{"stdio no command", ServerInput{Transport: "stdio", Command: ""}, true},
		{"http no endpoint", ServerInput{Transport: "streamable_http", Endpoint: ""}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NormalizeServerIdentity(c.input)
			if c.reject && err == nil {
				t.Fatalf("expected rejection for %s", c.name)
			}
			if !c.reject && err != nil {
				t.Fatalf("unexpected rejection for %s: %v", c.name, err)
			}
		})
	}
}

func TestLegacy_MCP_IdentityNormalization(t *testing.T) {
	identity, err := NormalizeServerIdentity(ServerInput{Transport: "streamable_http", Endpoint: "HTTPS://Example.COM/mcp#fragment"})
	if err != nil {
		t.Fatal(err)
	}
	if identity != "streamable_http:https://example.com/mcp" {
		t.Fatalf("unexpected identity: %s", identity)
	}

	identity, err = NormalizeServerIdentity(ServerInput{Transport: "stdio", Command: "  node  ", Args: []string{" -e ", "console.log(1)"}})
	if err != nil {
		t.Fatal(err)
	}
	if identity != `stdio:node:[" -e ","console.log(1)"]` {
		t.Fatalf("unexpected stdio identity: %s", identity)
	}
}

func TestLegacy_MCP_ServerListBaseline(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	_, err := repo.CreateServer(ctx, ServerInput{Name: "server-a", Transport: "streamable_http", Endpoint: "https://a.example.com/mcp", AuthType: "none", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateServer(ctx, ServerInput{Name: "server-b", Transport: "streamable_http", Endpoint: "https://b.example.com/mcp", AuthType: "none", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	all, err := repo.ListServers(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("expected 2 servers, got %d %v", len(all), err)
	}

	enabled, err := repo.ListEnabledServers(ctx)
	if err != nil || len(enabled) != 2 {
		t.Fatalf("expected 2 enabled, got %d %v", len(enabled), err)
	}
}

func TestLegacy_MCP_ScopeBinding(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.SetScopeEnabled(ctx, srv.ID, "global", "", true); err != nil {
		t.Fatal(err)
	}

	enabled, scopeType, err := repo.ResolveScopeEnabled(ctx, srv.ID, "")
	if err != nil || !enabled || scopeType != "global" {
		t.Fatalf("expected global enabled: %v %s %v", enabled, scopeType, err)
	}

	if err := repo.SetScopeEnabled(ctx, srv.ID, "character", "char-1", true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetScopeEnabled(ctx, srv.ID, "character", "char-1", false); err != nil {
		t.Fatal(err)
	}

	enabled, scopeType, err = repo.ResolveScopeEnabled(ctx, srv.ID, "char-1")
	if err != nil || enabled || scopeType != "character" {
		t.Fatalf("char-1 should be disabled after set false: %v %s %v", enabled, scopeType, err)
	}
}

func TestLegacy_MCP_ToolSyncAndList(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	tools := []ToolDefinition{
		{
			ID:                  "tool-1",
			ServerID:            srv.ID,
			RemoteName:          "search",
			SkillID:             "mcp.tool.test.search",
			Title:               "Search Tool",
			Description:         "Searches documents",
			InputSchemaJSON:     `{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`,
			OutputSchemaJSON:    `{"type":"object"}`,
			AnnotationsJSON:     `{}`,
			ExecutionJSON:       `{}`,
			CapabilityHintsJSON: `[]`,
			RiskLevel:           "low",
			Enabled:             1,
			Hash:                "abc123",
		},
	}
	if err := repo.SyncTools(ctx, srv.ID, tools); err != nil {
		t.Fatal(err)
	}

	listed, err := repo.ListTools(ctx, srv.ID, true)
	if err != nil || len(listed) != 1 {
		t.Fatalf("expected 1 tool, got %d %v", len(listed), err)
	}
	if listed[0].RemoteName != "search" || listed[0].SkillID != "mcp.tool.test.search" {
		t.Fatalf("unexpected tool: %+v", listed[0])
	}

	fetched, err := repo.GetToolBySkillID(ctx, "mcp.tool.test.search")
	if err != nil || fetched.RemoteName != "search" {
		t.Fatalf("fetch by skill ID failed: %+v %v", fetched, err)
	}

	if err := repo.SetToolEnabled(ctx, "tool-1", false); err != nil {
		t.Fatal(err)
	}
	disabled, err := repo.ListTools(ctx, srv.ID, true)
	if err != nil || len(disabled) != 0 {
		t.Fatalf("disabled tool should not appear when enabledOnly: %d %v", len(disabled), err)
	}

	all, err := repo.ListTools(ctx, srv.ID, false)
	if err != nil || len(all) != 1 {
		t.Fatalf("all tools should include disabled: %d %v", len(all), err)
	}
}

func TestLegacy_MCP_ResourceSyncAndList(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	resources := []ResourceDefinition{{
		ID:          "res-1",
		ServerID:    srv.ID,
		URI:         "file:///data/config.json",
		Name:        "config",
		Title:       "Configuration",
		Description: "Server config",
		MIMEType:    "application/json",
		Enabled:     1,
	}}
	if err := repo.SyncResources(ctx, srv.ID, resources, nil); err != nil {
		t.Fatal(err)
	}

	rList, tList, err := repo.ListResources(ctx, srv.ID, true)
	if err != nil || len(rList) != 1 || rList[0].Name != "config" {
		t.Fatalf("resource not persisted: resources=%d templates=%d %v", len(rList), len(tList), err)
	}
}

func TestLegacy_MCP_PromptSyncAndList(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()

	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	prompts := []PromptDefinition{{
		ID:            "prompt-1",
		ServerID:      srv.ID,
		RemoteName:    "greeting",
		Title:         "Greeting Prompt",
		Description:   "A greeting template",
		ArgumentsJSON: `[{"name":"name","required":true}]`,
		Enabled:       1,
	}}
	if err := repo.SyncPrompts(ctx, srv.ID, prompts); err != nil {
		t.Fatal(err)
	}

	listed, err := repo.ListPrompts(ctx, srv.ID, true)
	if err != nil || len(listed) != 1 || listed[0].RemoteName != "greeting" {
		t.Fatalf("prompt not persisted: %+v %v", listed, err)
	}

	found, err := repo.GetPromptByName(ctx, srv.ID, "greeting")
	if err != nil || found.Title != "Greeting Prompt" {
		t.Fatalf("prompt lookup by name failed: %+v %v", found, err)
	}
}

func TestLegacy_MCP_ServerCapabilityBaseline(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()
	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	capabilities := []string{"roots", "sampling", "elicitation", "tasks", "private_network"}
	for _, cap := range capabilities {
		enabled, config, err := repo.ServerCapabilityEnabled(ctx, srv.ID, cap)
		if err != nil || enabled || string(config) != "{}" {
			t.Fatalf("capability %s unexpected default: enabled=%v config=%s err=%v", cap, enabled, config, err)
		}
	}

	if _, err := repo.SetServerCapability(ctx, srv.ID, "roots", true, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetServerCapability(ctx, srv.ID, "sampling", true, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	records, err := repo.ListServerCapabilities(ctx, srv.ID)
	if err != nil || len(records) != 2 {
		t.Fatalf("expected 2 capabilities, got %d %v", len(records), err)
	}

	if _, err := repo.SetServerCapability(ctx, srv.ID, "unknown", true, json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected unknown capability rejection")
	}
	if _, err := repo.SetServerCapability(ctx, srv.ID, "tasks", true, json.RawMessage(`[]`)); err == nil {
		t.Fatal("expected non-object config rejection")
	}
}

func TestLegacy_MCP_CredentialReferenceLifecycle(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()
	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	refs, err := repo.CredentialReferences(ctx, srv.ID)
	if err != nil || len(refs) != 0 {
		t.Fatalf("expected no credential refs, got %d %v", len(refs), err)
	}

	if _, err := repo.PutCredentialReference(ctx, srv.ID, "oauth2", "secret-ref-1", "", []string{"tools"}); err != nil {
		t.Fatal(err)
	}

	cred, err := repo.CredentialReference(ctx, srv.ID, "oauth2")
	if err != nil || cred.SecretReference != "secret-ref-1" {
		t.Fatalf("credential ref mismatch: %+v %v", cred, err)
	}

	deleted, err := repo.DeleteCredentialReferences(ctx, srv.ID)
	if err != nil || len(deleted) != 1 {
		t.Fatalf("credential not deleted: %+v %v", deleted, err)
	}
}

func TestLegacy_MCP_OAuthSessionLifecycle(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()
	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}

	state := "raw-state-value"
	hashed := auth.HashState(state)
	session := auth.PendingSession{
		ID:                    "session-1",
		ServerID:              srv.ID,
		StateHash:             hashed,
		CodeVerifierReference: "verifier-1",
		RedirectURI:           "http://localhost/callback",
		Status:                "pending",
		ExpiresAt:             time.Now().Add(time.Minute),
	}
	if err := repo.CreateOAuthSession(ctx, session); err != nil {
		t.Fatal(err)
	}

	found, err := repo.FindOAuthSessionByStateHash(ctx, hashed)
	if err != nil || found.ID != "session-1" || found.ServerID != srv.ID {
		t.Fatalf("session not found by state hash: %+v %v", found, err)
	}

	consumed, err := repo.ConsumeOAuthSession(ctx, "session-1", hashed)
	if err != nil || consumed.Status != "pending" {
		t.Fatalf("session not consumed: %+v %v", consumed, err)
	}

	if _, err := repo.ConsumeOAuthSession(ctx, "session-1", hashed); err == nil {
		t.Fatal("session replay should be rejected")
	}
}

func TestLegacy_MCP_ServerStatusTransitions(t *testing.T) {
	repo := NewRepository(mcpBaselineDB(t))
	ctx := context.Background()
	srv, err := repo.CreateServer(ctx, ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Status != "disconnected" {
		t.Fatalf("new server should be disconnected: %s", srv.Status)
	}

	if err := repo.SetServerStatus(ctx, srv.ID, "connected", "", "", nil); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.GetServer(ctx, srv.ID)
	if updated.Status != "connected" {
		t.Fatalf("status not updated to connected: %s", updated.Status)
	}

	if err := repo.SetServerStatus(ctx, srv.ID, "error", "TRANSPORT_ERROR", "connection refused", nil); err != nil {
		t.Fatal(err)
	}
	updated, _ = repo.GetServer(ctx, srv.ID)
	if updated.Status != "error" || updated.LastErrorCode != "TRANSPORT_ERROR" {
		t.Fatalf("error status not persisted: %s %s", updated.Status, updated.LastErrorCode)
	}
}

func TestLegacy_MCP_AlternateCommandPortability(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		args     []string
		reject   bool
		expected string
	}{
		{"node", "node", []string{"server.js"}, false, `stdio:node:["server.js"]`},
		{"python", "python3", []string{"-m", "mcp.server"}, false, `stdio:python3:["-m","mcp.server"]`},
		{"empty command", "", nil, true, ""},
		{"spaces trimmed", "  npx  ", []string{"-y"}, false, `stdio:npx:["-y"]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			identity, err := NormalizeServerIdentity(ServerInput{Transport: "stdio", Command: c.command, Args: c.args})
			if c.reject && err == nil {
				t.Fatal("expected rejection")
			}
			if !c.reject {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if identity != c.expected {
					t.Fatalf("expected %s, got %s", c.expected, identity)
				}
			}
		})
	}
}
