package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/mcp/auth"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func capabilityTestRepository(t *testing.T) (*Repository, Server) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Server{}, &ServerCapability{}, &OAuthSession{}); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	server, err := repository.CreateServer(context.Background(), ServerInput{Name: "test", Transport: "streamable_http", Endpoint: "https://example.com/mcp", AuthType: "none"})
	if err != nil {
		t.Fatal(err)
	}
	return repository, server
}

func TestServerCapabilityDefaultsDisabledAndUpserts(t *testing.T) {
	repository, server := capabilityTestRepository(t)
	enabled, configuration, err := repository.ServerCapabilityEnabled(context.Background(), server.ID, "sampling")
	if err != nil || enabled || string(configuration) != "{}" {
		t.Fatalf("unexpected default: enabled=%v config=%s err=%v", enabled, configuration, err)
	}
	if _, err := repository.SetServerCapability(context.Background(), server.ID, "sampling", true, json.RawMessage(`{"maxTokens":1024}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SetServerCapability(context.Background(), server.ID, "sampling", false, json.RawMessage(`{"maxTokens":512}`)); err != nil {
		t.Fatal(err)
	}
	records, err := repository.ListServerCapabilities(context.Background(), server.ID)
	if err != nil || len(records) != 1 || records[0].Enabled != 0 || records[0].Configuration != `{"maxTokens":512}` {
		t.Fatalf("unexpected records: %#v err=%v", records, err)
	}
}

func TestServerCapabilityRejectsUnknownAndNonObjectConfiguration(t *testing.T) {
	repository, server := capabilityTestRepository(t)
	if _, err := repository.SetServerCapability(context.Background(), server.ID, "unknown", true, json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected unknown capability error")
	}
	if _, err := repository.SetServerCapability(context.Background(), server.ID, "roots", true, json.RawMessage(`[]`)); err == nil {
		t.Fatal("expected non-object configuration error")
	}
}

func TestNormalizeServerIdentityRejectsUnsafeEndpointsAndMissingCommands(t *testing.T) {
	cases := []ServerInput{{Transport: "streamable_http", Endpoint: "ftp://example.com/mcp"}, {Transport: "streamable_http", Endpoint: "https://user:pass@example.com/mcp"}, {Transport: "stdio", Command: ""}, {Transport: "unsupported"}}
	for _, input := range cases {
		if _, err := NormalizeServerIdentity(input); err == nil {
			t.Fatalf("expected rejection: %#v", input)
		}
	}
	identity, err := NormalizeServerIdentity(ServerInput{Transport: "streamable_http", Endpoint: "HTTPS://Example.COM/mcp#fragment"})
	if err != nil || identity != "streamable_http:https://example.com/mcp" {
		t.Fatalf("unexpected identity=%q err=%v", identity, err)
	}
}

func TestOAuthSessionCanBeLocatedByStateAndConsumedOnce(t *testing.T) {
	repository, server := capabilityTestRepository(t)
	state := "raw-state"
	session := auth.PendingSession{ID: "session-1", ServerID: server.ID, StateHash: auth.HashState(state), CodeVerifierReference: "secret-verifier", RedirectURI: "http://127.0.0.1/callback", Status: "pending", ExpiresAt: time.Now().Add(time.Minute)}
	if err := repository.CreateOAuthSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindOAuthSessionByStateHash(context.Background(), auth.HashState(state))
	if err != nil || found.ID != session.ID || found.ServerID != server.ID {
		t.Fatalf("unexpected session=%#v err=%v", found, err)
	}
	consumed, err := repository.ConsumeOAuthSession(context.Background(), "", auth.HashState(state))
	if err != nil || consumed.ID != session.ID {
		t.Fatalf("unexpected consume=%#v err=%v", consumed, err)
	}
	if _, err := repository.ConsumeOAuthSession(context.Background(), "", auth.HashState(state)); err == nil {
		t.Fatal("expected replay rejection")
	}
}
