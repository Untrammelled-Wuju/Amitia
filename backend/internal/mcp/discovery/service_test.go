package discovery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/client"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type discoveryCaller struct {
	responses map[string][]json.RawMessage
	calls     map[string]int
}

func (c *discoveryCaller) Call(_ context.Context, _ string, method string, _ any, _ client.CallOptions) (json.RawMessage, error) {
	index := c.calls[method]
	c.calls[method] = index + 1
	values := c.responses[method]
	if index >= len(values) {
		return json.RawMessage(`{}`), nil
	}
	return values[index], nil
}
func (c *discoveryCaller) Connection(string) (*client.Connection, bool) { return nil, false }

func discoveryTestService(t *testing.T) (*Service, *mcp.Repository, *discoveryCaller, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&mcp.Server{}, &mcp.ToolDefinition{}, &mcp.ResourceDefinition{}, &mcp.ResourceTemplate{}, &mcp.PromptDefinition{}); err != nil {
		t.Fatal(err)
	}
	repository := mcp.NewRepository(db)
	server, err := repository.CreateServer(context.Background(), mcp.ServerInput{Name: "discovery", Transport: "streamable_http", Endpoint: "https://example.com/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	initialized := &struct{ ProtocolVersion, ServerInfoJSON, CapabilitiesJSON, Instructions string }{"2025-11-25", `{}`, `{"tools":{},"resources":{},"prompts":{}}`, ""}
	if err := repository.SetServerStatus(context.Background(), server.ID, "ready", "", "", initialized); err != nil {
		t.Fatal(err)
	}
	caller := &discoveryCaller{responses: map[string][]json.RawMessage{}, calls: map[string]int{}}
	return New(repository, caller), repository, caller, server.ID
}

func TestDiscoverPaginatesAndPersistsAllCapabilities(t *testing.T) {
	service, repository, caller, serverID := discoveryTestService(t)
	caller.responses["tools/list"] = []json.RawMessage{json.RawMessage(`{"tools":[{"name":"first","inputSchema":{"type":"object"},"annotations":{"readOnlyHint":true}}],"nextCursor":"next"}`), json.RawMessage(`{"tools":[{"name":"second","inputSchema":{"type":"object"},"outputSchema":{"type":"object"}}]}`)}
	caller.responses["resources/list"] = []json.RawMessage{json.RawMessage(`{"resources":[{"uri":"test://resource","name":"resource","mimeType":"text/plain"}]}`)}
	caller.responses["resources/templates/list"] = []json.RawMessage{json.RawMessage(`{"resourceTemplates":[{"uriTemplate":"test://{id}","name":"template"}]}`)}
	caller.responses["prompts/list"] = []json.RawMessage{json.RawMessage(`{"prompts":[{"name":"prompt","arguments":[{"name":"topic","required":true}]}]}`)}
	if err := service.Discover(context.Background(), serverID); err != nil {
		t.Fatal(err)
	}
	tools, err := repository.ListTools(context.Background(), serverID, false)
	if err != nil || len(tools) != 2 || caller.calls["tools/list"] != 2 {
		t.Fatalf("unexpected tools=%#v calls=%d err=%v", tools, caller.calls["tools/list"], err)
	}
	resources, templates, err := repository.ListResources(context.Background(), serverID, false)
	if err != nil || len(resources) != 1 || len(templates) != 1 {
		t.Fatalf("unexpected resources=%#v templates=%#v err=%v", resources, templates, err)
	}
	prompts, err := repository.ListPrompts(context.Background(), serverID, false)
	if err != nil || len(prompts) != 1 {
		t.Fatalf("unexpected prompts=%#v err=%v", prompts, err)
	}
}

func TestDiscoverRejectsInvalidSchemaAndCursorCycle(t *testing.T) {
	service, _, caller, serverID := discoveryTestService(t)
	caller.responses["tools/list"] = []json.RawMessage{json.RawMessage(`{"tools":[{"name":"bad","inputSchema":{"type":"string"}}]}`)}
	if err := service.Discover(context.Background(), serverID); err == nil {
		t.Fatal("expected invalid schema error")
	}
	caller.calls = map[string]int{}
	caller.responses["tools/list"] = []json.RawMessage{json.RawMessage(`{"tools":[],"nextCursor":"same"}`), json.RawMessage(`{"tools":[],"nextCursor":"same"}`)}
	if err := service.Discover(context.Background(), serverID); err == nil {
		t.Fatal("expected cursor cycle error")
	}
}

func TestStableSkillIDUsesCollisionResistantSuffix(t *testing.T) {
	left := StableSkillID("server", "tool-"+strings.Repeat("a", 80)+"left")
	right := StableSkillID("server", "tool-"+strings.Repeat("a", 80)+"right")
	if left == right || len(left) > 85 || len(right) > 85 {
		t.Fatalf("unexpected ids %q %q", left, right)
	}
}
