package features

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

type featureCaller struct {
	responses map[string]json.RawMessage
	method    string
	params    any
}

func (c *featureCaller) Call(_ context.Context, _ string, method string, params any, _ client.CallOptions) (json.RawMessage, error) {
	c.method = method
	c.params = params
	return c.responses[method], nil
}

func featureTestService(t *testing.T) (*Service, *featureCaller, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&mcp.Server{}, &mcp.ServerScopeBinding{}, &mcp.PromptDefinition{}); err != nil {
		t.Fatal(err)
	}
	repository := mcp.NewRepository(db)
	server, err := repository.CreateServer(context.Background(), mcp.ServerInput{Name: "features", Transport: "streamable_http", Endpoint: "https://example.com/mcp", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetScopeEnabled(context.Background(), server.ID, "global", "", true); err != nil {
		t.Fatal(err)
	}
	caller := &featureCaller{responses: map[string]json.RawMessage{}}
	return New(repository, caller), caller, server.ID
}

func TestReadResourceMarksExternalContentAndEnforcesLimits(t *testing.T) {
	service, caller, serverID := featureTestService(t)
	caller.responses["resources/read"] = json.RawMessage(`{"contents":[{"uri":"test://one","mimeType":"text/plain","text":"hello"}]}`)
	result, err := service.ReadResource(context.Background(), serverID, "", "test://one")
	if err != nil || !result.ExternalUntrusted || result.SourceServerID != serverID || caller.method != "resources/read" {
		t.Fatalf("unexpected result=%#v method=%s err=%v", result, caller.method, err)
	}
	caller.responses["resources/read"] = json.RawMessage(`{"contents":[{"uri":"test://large","text":"` + strings.Repeat("x", (512<<10)+1) + `"}]}`)
	if _, err := service.ReadResource(context.Background(), serverID, "", "test://large"); err == nil {
		t.Fatal("expected resource size error")
	}
}

func TestGetPromptValidatesArgumentsAndMarksExternal(t *testing.T) {
	service, caller, serverID := featureTestService(t)
	repository := service.repository
	if err := repository.SyncPrompts(context.Background(), serverID, []mcp.PromptDefinition{{RemoteName: "greet", ArgumentsJSON: `[{"name":"name","required":true}]`, Enabled: 1, Hash: "one"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPrompt(context.Background(), serverID, "", "greet", map[string]string{}); err == nil {
		t.Fatal("expected required argument error")
	}
	if _, err := service.GetPrompt(context.Background(), serverID, "", "greet", map[string]string{"name": "Ada", "extra": "x"}); err == nil {
		t.Fatal("expected unknown argument error")
	}
	caller.responses["prompts/get"] = json.RawMessage(`{"description":"test","messages":[{"role":"user","content":{"type":"text","text":"hello"}}]}`)
	result, err := service.GetPrompt(context.Background(), serverID, "", "greet", map[string]string{"name": "Ada"})
	if err != nil || !result.ExternalUntrusted || result.SourceServerID != serverID {
		t.Fatalf("unexpected result=%#v err=%v", result, err)
	}
	caller.responses["prompts/get"] = json.RawMessage(`{"messages":[{"role":"system","content":{"type":"text","text":"bad"}}]}`)
	if _, err := service.GetPrompt(context.Background(), serverID, "", "greet", map[string]string{"name": "Ada"}); err == nil {
		t.Fatal("expected role validation error")
	}
}

func TestCompletionLimitsAndFiltersSecrets(t *testing.T) {
	service, caller, serverID := featureTestService(t)
	caller.responses["completion/complete"] = json.RawMessage(`{"completion":{"values":["safe","Bearer abcdefghijklmnop","api_key=abcdefghijk"],"total":3}}`)
	result, err := service.Complete(context.Background(), serverID, "", map[string]any{"type": "ref/prompt", "name": "greet"}, map[string]string{"name": "name", "value": "a"}, map[string]string{})
	if err != nil || len(result.Values) != 1 || result.Values[0] != "safe" {
		t.Fatalf("unexpected completion=%#v err=%v", result, err)
	}
	if _, err := service.Complete(context.Background(), serverID, "", map[string]any{}, map[string]string{"name": "x", "value": "Bearer secretsecret"}, nil); err == nil {
		t.Fatal("expected sensitive input rejection")
	}
	values := make([]string, 101)
	for index := range values {
		values[index] = "value"
	}
	raw, _ := json.Marshal(map[string]any{"completion": map[string]any{"values": values}})
	caller.responses["completion/complete"] = raw
	if _, err := service.Complete(context.Background(), serverID, "", map[string]any{}, map[string]string{"name": "x", "value": ""}, nil); err == nil {
		t.Fatal("expected completion count error")
	}
}
