package sandbox_webui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestBridge(t *testing.T) (*Host, *Bridge, *WebSession) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	dispatcher := NewBridgeActionDispatcher(func(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"ok": true, "actionId": actionID})
	})
	provider := NewBridgeDataSourceProvider()
	nav := NewBridgeNavigator(nil)
	bridge := NewBridge(host, dispatcher, provider, nav)
	host.SetBridge(bridge)

	result, err := host.CreateSession(CreateSessionRequest{
		ExtensionID:    "ext-1",
		ModuleID:       "mod-1",
		Generation:     1,
		SlotID:         "slot-1",
		Sandbox:        SandboxWebRestricted,
		EntryPath:      "index.html",
		BasePath:       tmpDir,
		AllowedActions: []string{"test-action"},
	})
	if err != nil {
		t.Fatal(err)
	}

	session, err := host.GetSession(result.SessionID)
	if err != nil {
		t.Fatal(err)
	}

	return host, bridge, session
}

func TestBridgeHandleReady(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	result, err := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodReady),
			Version: 1,
			ID:      "msg-1",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if result.Error != "" {
		t.Errorf("expected no error, got %s", result.Error)
	}
}

func TestBridgeHandleContextGet(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	result, err := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodContextGet),
			Version: 1,
			ID:      "msg-2",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if len(result.Output) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestBridgeHandlePing(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	result, err := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodSessionPing),
			Version: 1,
			ID:      "msg-3",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if len(result.Output) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestBridgeHandleActionInvoke(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	input, _ := json.Marshal(map[string]any{
		"actionId": "test-action",
		"input":    nil,
	})

	result, err := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodActionInvoke),
			Version: 1,
			ID:      "msg-4",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
			Input:   input,
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if len(result.Output) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestBridgeHandleResize(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	input, _ := json.Marshal(map[string]any{
		"width":  800,
		"height": 600,
	})

	result, err := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodResize),
			Version: 1,
			ID:      "msg-5",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
			Input:   input,
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if result.Error != "" {
		t.Errorf("expected no error, got %s", result.Error)
	}
}

func TestBridgeHandleInvalidResize(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	input, _ := json.Marshal(map[string]any{
		"width":  50,
		"height": 50,
	})

	result, _ := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodResize),
			Version: 1,
			ID:      "msg-6",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
			Input:   input,
		},
	})
	if result.Error == "" {
		t.Error("expected error for invalid resize dimensions")
	}
}

func TestBridgeRateLimit(t *testing.T) {
	_, bridge, session := setupTestBridge(t)

	for i := 0; i < MaxLogPerSec; i++ {
		bridge.HandleMessage(context.Background(), InvokeRequest{
			SessionID: session.SessionID,
			Message: BridgeMessage{
				Method:  string(MethodLog),
				Version: 1,
				ID:      "msg-log",
				Session: session.SessionID,
				Origin:  session.Origin,
				Nonce:   session.Nonce,
			},
		})
	}

	result, _ := bridge.HandleMessage(context.Background(), InvokeRequest{
		SessionID: session.SessionID,
		Message: BridgeMessage{
			Method:  string(MethodLog),
			Version: 1,
			ID:      "msg-log-excess",
			Session: session.SessionID,
			Origin:  session.Origin,
			Nonce:   session.Nonce,
		},
	})
	if result.Error == "" {
		t.Error("expected rate limit error")
	}
}

func TestValidateBridgeMessage(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.html")
	os.WriteFile(entryFile, []byte("<html></html>"), 0644)

	host := NewHost()
	result, _ := host.CreateSession(CreateSessionRequest{
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		Generation:  1,
		SlotID:      "slot-1",
		Sandbox:     SandboxWebRestricted,
		EntryPath:   "index.html",
		BasePath:    tmpDir,
	})

	session, _ := host.GetSession(result.SessionID)

	msg := &BridgeMessage{
		Method:  string(MethodReady),
		Session: session.SessionID,
		Origin:  session.Origin,
		Nonce:   session.Nonce,
	}
	if err := ValidateBridgeMessage(msg, session); err != nil {
		t.Errorf("valid message should pass: %v", err)
	}

	msg.Nonce = "wrong-nonce"
	if err := ValidateBridgeMessage(msg, session); err != ErrNonceMismatch {
		t.Errorf("expected ErrNonceMismatch, got %v", err)
	}

	msg.Nonce = session.Nonce
	msg.Session = "wrong-session"
	if err := ValidateBridgeMessage(msg, session); err != ErrSessionMismatch {
		t.Errorf("expected ErrSessionMismatch, got %v", err)
	}
}
