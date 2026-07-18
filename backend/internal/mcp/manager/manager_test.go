package manager

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/client"
	"github.com/u-ai/backend/internal/mcp/protocol"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func managerCapabilityTest(t *testing.T) (*Manager, *mcp.Repository, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&mcp.Server{}, &mcp.ServerCapability{}); err != nil {
		t.Fatal(err)
	}
	repository := mcp.NewRepository(db)
	server, err := repository.CreateServer(context.Background(), mcp.ServerInput{Name: "manager", Transport: "streamable_http", Endpoint: "https://example.com/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	manager := New(repository, nil, Config{Connection: client.Config{Capabilities: protocol.ClientCapabilities{Roots: map[string]any{"listChanged": true}, Sampling: map[string]any{}, Elicitation: map[string]any{}, Tasks: map[string]any{}}}})
	return manager, repository, server.ID
}

func TestClientCapabilitiesDefaultOffAndPerServerEnabled(t *testing.T) {
	manager, repository, serverID := managerCapabilityTest(t)
	initial := manager.clientCapabilities(context.Background(), serverID)
	if initial.Roots != nil || initial.Sampling != nil || initial.Elicitation != nil || initial.Tasks != nil {
		t.Fatalf("capabilities should default off: %#v", initial)
	}
	if _, err := repository.SetServerCapability(context.Background(), serverID, "roots", true, json.RawMessage(`{"roots":[]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SetServerCapability(context.Background(), serverID, "sampling", true, json.RawMessage(`{"maxTokens":512}`)); err != nil {
		t.Fatal(err)
	}
	enabled := manager.clientCapabilities(context.Background(), serverID)
	if enabled.Roots == nil || enabled.Roots["listChanged"] != true || enabled.Sampling == nil || enabled.Elicitation != nil || enabled.Tasks != nil {
		t.Fatalf("unexpected capabilities: %#v", enabled)
	}
}
