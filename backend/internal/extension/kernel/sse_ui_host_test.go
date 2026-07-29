package kernel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/sse"
)

func TestDefaultUIHostNotifier_FailClosed(t *testing.T) {
	n := NewDefaultUIHostNotifier()

	if err := n.Notify(context.Background(), "ext-1", "title", "body", "info"); !errors.Is(err, ErrUIHostUnavailable) {
		t.Errorf("Notify() error = %v, want ErrUIHostUnavailable", err)
	}

	if _, err := n.Dialog(context.Background(), "ext-1", "dlg-1", "msg", []string{"OK"}); !errors.Is(err, ErrDialogHostUnavailable) {
		t.Errorf("Dialog() error = %v, want ErrDialogHostUnavailable", err)
	}

	if err := n.Navigate(context.Background(), "ext-1", "/target"); !errors.Is(err, ErrNavigationHostUnavailable) {
		t.Errorf("Navigate() error = %v, want ErrNavigationHostUnavailable", err)
	}
}

func TestSSEUIHostNotifier_HostUnavailableWithoutClients(t *testing.T) {
	hub := &sse.Hub{}
	n := NewSSEUIHostNotifier(hub)

	if err := n.Notify(context.Background(), "ext-1", "title", "body", "info"); !errors.Is(err, ErrUIHostUnavailable) {
		t.Errorf("Notify() error = %v, want ErrUIHostUnavailable", err)
	}

	if _, err := n.Dialog(context.Background(), "ext-1", "dlg-1", "msg", []string{"OK"}); !errors.Is(err, ErrDialogHostUnavailable) {
		t.Errorf("Dialog() error = %v, want ErrDialogHostUnavailable", err)
	}

	if err := n.Navigate(context.Background(), "ext-1", "/target"); !errors.Is(err, ErrNavigationHostUnavailable) {
		t.Errorf("Navigate() error = %v, want ErrNavigationHostUnavailable", err)
	}
}

func TestSSEUIHostNotifier_NilHub(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)

	if err := n.Notify(context.Background(), "ext-1", "title", "body", "info"); !errors.Is(err, ErrUIHostUnavailable) {
		t.Errorf("Notify() error = %v, want ErrUIHostUnavailable", err)
	}

	if _, err := n.Dialog(context.Background(), "ext-1", "dlg-1", "msg", []string{"OK"}); !errors.Is(err, ErrDialogHostUnavailable) {
		t.Errorf("Dialog() error = %v, want ErrDialogHostUnavailable", err)
	}

	if err := n.Navigate(context.Background(), "ext-1", "/target"); !errors.Is(err, ErrNavigationHostUnavailable) {
		t.Errorf("Navigate() error = %v, want ErrNavigationHostUnavailable", err)
	}
}

func TestSSEUIHostNotifier_NotifyWithClients(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-notify-" + t.Name())
	defer hub.Unsubscribe("test-notify-" + t.Name())

	n := NewSSEUIHostNotifier(hub)
	if err := n.Notify(context.Background(), "ext-1", "Test Title", "Test Body", "info"); err != nil {
		t.Fatalf("Notify() error = %v, want nil", err)
	}

	select {
	case msg := <-client.Events:
		event, ok := msg["event"].(string)
		if !ok || event != "ui_notify" {
			t.Errorf("expected event ui_notify, got %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be map[string]interface{}")
		}
		if data["eventType"] != "ui_notify" {
			t.Errorf("expected eventType 'ui_notify', got %v", data["eventType"])
		}
		if data["extensionId"] != "ext-1" {
			t.Errorf("expected extensionId 'ext-1', got %v", data["extensionId"])
		}
		if data["requestId"] == nil || data["requestId"] == "" {
			t.Error("expected requestId to be non-empty")
		}
		if data["sessionId"] != "ui-host" {
			t.Errorf("expected sessionId 'ui-host', got %v", data["sessionId"])
		}
		if data["expiresAt"] == nil || data["expiresAt"] == "" {
			t.Error("expected expiresAt to be non-empty")
		}
		if data["timestamp"] == nil || data["timestamp"] == "" {
			t.Error("expected timestamp to be non-empty")
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			t.Fatal("expected payload to be map[string]interface{}")
		}
		if payload["title"] != "Test Title" {
			t.Errorf("expected payload title 'Test Title', got %v", payload["title"])
		}
		if payload["body"] != "Test Body" {
			t.Errorf("expected payload body 'Test Body', got %v", payload["body"])
		}
		if payload["severity"] != "info" {
			t.Errorf("expected payload severity 'info', got %v", payload["severity"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestSSEUIHostNotifier_NavigateWithClients(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-navigate-" + t.Name())
	defer hub.Unsubscribe("test-navigate-" + t.Name())

	n := NewSSEUIHostNotifier(hub)
	if err := n.Navigate(context.Background(), "ext-1", "/chat"); err != nil {
		t.Fatalf("Navigate() error = %v, want nil", err)
	}

	select {
	case msg := <-client.Events:
		event, ok := msg["event"].(string)
		if !ok || event != "ui_navigate" {
			t.Errorf("expected event ui_navigate, got %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be map[string]interface{}")
		}
		if data["eventType"] != "ui_navigate" {
			t.Errorf("expected eventType 'ui_navigate', got %v", data["eventType"])
		}
		if data["requestId"] == nil || data["requestId"] == "" {
			t.Error("expected requestId to be non-empty")
		}
		if data["expiresAt"] == nil || data["expiresAt"] == "" {
			t.Error("expected expiresAt to be non-empty")
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			t.Fatal("expected payload to be map[string]interface{}")
		}
		if payload["target"] != "/chat" {
			t.Errorf("expected payload target '/chat', got %v", payload["target"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestSSEUIHostNotifier_DialogWithResolution(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-dialog-" + t.Name())
	defer hub.Unsubscribe("test-dialog-" + t.Name())

	n := NewSSEUIHostNotifier(hub)

	dialogID := "test-dlg-resolve"
	resultCh := make(chan struct {
		result string
		err    error
	}, 1)

	go func() {
		result, err := n.Dialog(context.Background(), "ext-1", dialogID, "Choose", []string{"Yes", "No"})
		resultCh <- struct {
			result string
			err    error
		}{result, err}
	}()

	select {
	case msg := <-client.Events:
		event, ok := msg["event"].(string)
		if !ok || event != "ui_dialog" {
			t.Errorf("expected event ui_dialog, got %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be map[string]interface{}")
		}
		if data["eventType"] != "ui_dialog" {
			t.Errorf("expected eventType 'ui_dialog', got %v", data["eventType"])
		}
		if data["requestId"] == nil || data["requestId"] == "" {
			t.Error("expected requestId to be non-empty")
		}
		if data["expiresAt"] == nil || data["expiresAt"] == "" {
			t.Error("expected expiresAt to be non-empty")
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			t.Fatal("expected payload to be map[string]interface{}")
		}
		if payload["dialogId"] != dialogID {
			t.Errorf("expected payload dialogId '%s', got %v", dialogID, payload["dialogId"])
		}
		if payload["message"] != "Choose" {
			t.Errorf("expected payload message 'Choose', got %v", payload["message"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE event")
	}

	if !n.HasPendingDialog(dialogID) {
		t.Fatal("expected pending dialog to exist")
	}

	if !n.ResolveDialog(dialogID, "Yes") {
		t.Fatal("ResolveDialog returned false, expected true")
	}

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Fatalf("Dialog() error = %v, want nil", r.err)
		}
		if r.result != "Yes" {
			t.Errorf("Dialog() result = %v, want 'Yes'", r.result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dialog result")
	}

	if n.HasPendingDialog(dialogID) {
		t.Fatal("expected pending dialog to be removed after resolution")
	}
}

func TestSSEUIHostNotifier_DialogTimeout(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-dialog-timeout-" + t.Name())
	defer hub.Unsubscribe("test-dialog-timeout-" + t.Name())

	n := NewSSEUIHostNotifier(hub)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := n.Dialog(ctx, "ext-1", "test-dlg-timeout", "msg", []string{"OK"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	go func() {
		<-client.Events
	}()
}

func TestSSEUIHostNotifier_ResolveDialogNotFound(t *testing.T) {
	hub := &sse.Hub{}
	n := NewSSEUIHostNotifier(hub)

	if n.ResolveDialog("nonexistent", "ok") {
		t.Fatal("ResolveDialog returned true for non-existent dialog, expected false")
	}

	if n.FailDialog("nonexistent", errors.New("test")) {
		t.Fatal("FailDialog returned true for non-existent dialog, expected false")
	}
}

func TestSSEEventEnvelope_Fields(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	envelope := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)

	if envelope.EventType != "ui_notify" {
		t.Errorf("expected EventType 'ui_notify', got %v", envelope.EventType)
	}
	if envelope.ExtensionID != "ext-1" {
		t.Errorf("expected ExtensionID 'ext-1', got %v", envelope.ExtensionID)
	}
	if envelope.SessionID != "ui-host" {
		t.Errorf("expected SessionID 'ui-host', got %v", envelope.SessionID)
	}
	if envelope.RequestID == "" {
		t.Error("expected RequestID to be non-empty")
	}
	if envelope.Timestamp == "" {
		t.Error("expected Timestamp to be non-empty")
	}
	if envelope.ExpiresAt == "" {
		t.Error("expected ExpiresAt to be non-empty")
	}

	expiresAt, err := time.Parse(time.RFC3339, envelope.ExpiresAt)
	if err != nil {
		t.Fatalf("failed to parse ExpiresAt: %v", err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Error("expected ExpiresAt to be in the future")
	}
}

func TestSSEEventEnvelope_UniqueRequestIDs(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	e1 := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)
	e2 := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)
	if e1.RequestID == e2.RequestID {
		t.Error("expected RequestIDs to be unique")
	}
}

func TestSSEEventEnvelope_ToMap(t *testing.T) {
	payload := map[string]interface{}{"title": "test"}
	envelope := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)
	m := envelope.ToMap()

	if m["eventType"] != "ui_notify" {
		t.Errorf("expected eventType 'ui_notify', got %v", m["eventType"])
	}
	if m["extensionId"] != "ext-1" {
		t.Errorf("expected extensionId 'ext-1', got %v", m["extensionId"])
	}
	if m["requestId"] == "" {
		t.Error("expected requestId to be non-empty")
	}
	if m["payload"] == nil {
		t.Error("expected payload to be non-nil")
	}
	if m["expiresAt"] == "" {
		t.Error("expected expiresAt to be non-empty")
	}
	if m["timestamp"] == "" {
		t.Error("expected timestamp to be non-empty")
	}
}
