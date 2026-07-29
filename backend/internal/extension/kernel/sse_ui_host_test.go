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
		if data["title"] != "Test Title" {
			t.Errorf("expected title 'Test Title', got %v", data["title"])
		}
		if data["body"] != "Test Body" {
			t.Errorf("expected body 'Test Body', got %v", data["body"])
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
		if data["target"] != "/chat" {
			t.Errorf("expected target '/chat', got %v", data["target"])
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
		if data["dialogId"] != dialogID {
			t.Errorf("expected dialogId '%s', got %v", dialogID, data["dialogId"])
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
