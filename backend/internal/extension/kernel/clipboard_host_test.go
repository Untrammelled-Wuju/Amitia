package kernel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/sse"
)

func TestDefaultClipboardHost_FailClosed(t *testing.T) {
	h := NewDefaultClipboardHost()
	if err := h.WriteText(context.Background(), "test"); !errors.Is(err, ErrClipboardHostUnavailable) {
		t.Errorf("WriteText() error = %v, want ErrClipboardHostUnavailable", err)
	}
	if _, err := h.ReadText(context.Background()); !errors.Is(err, ErrClipboardHostUnavailable) {
		t.Errorf("ReadText() error = %v, want ErrClipboardHostUnavailable", err)
	}
}

func TestBridgeClipboardHost_NilHub(t *testing.T) {
	h := NewBridgeClipboardHost(nil)
	if err := h.WriteText(context.Background(), "test"); !errors.Is(err, ErrClipboardHostUnavailable) {
		t.Errorf("WriteText() error = %v, want ErrClipboardHostUnavailable", err)
	}
	if _, err := h.ReadText(context.Background()); !errors.Is(err, ErrClipboardHostUnavailable) {
		t.Errorf("ReadText() error = %v, want ErrClipboardHostUnavailable", err)
	}
	if h.IsAvailable() {
		t.Error("IsAvailable() = true, want false")
	}
}

func TestBridgeClipboardHost_NoClients(t *testing.T) {
	hub := &sse.Hub{}
	h := NewBridgeClipboardHost(hub)
	if err := h.WriteText(context.Background(), "test"); !errors.Is(err, ErrClipboardHostUnavailable) {
		t.Errorf("WriteText() error = %v, want ErrClipboardHostUnavailable", err)
	}
	if _, err := h.ReadText(context.Background()); !errors.Is(err, ErrClipboardHostUnavailable) {
		t.Errorf("ReadText() error = %v, want ErrClipboardHostUnavailable", err)
	}
	if h.IsAvailable() {
		t.Error("IsAvailable() = true, want false")
	}
}

func TestBridgeClipboardHost_WriteTextSuccess(t *testing.T) {
	hub := sse.Global
	clientID := "test-clip-write-" + t.Name()
	client := hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)

	h := NewBridgeClipboardHost(hub)
	if !h.IsAvailable() {
		t.Fatal("IsAvailable() = false, want true")
	}

	type result struct{ err error }
	resultCh := make(chan result, 1)
	go func() {
		err := h.WriteText(context.Background(), "hello clipboard")
		resultCh <- result{err: err}
	}()

	select {
	case msg := <-client.Events:
		event, _ := msg["event"].(string)
		if event != "clipboard_request" {
			t.Fatalf("expected event clipboard_request, got %s", event)
		}
		data, _ := msg["data"].(map[string]interface{})
		requestID, _ := data["requestId"].(string)
		if requestID == "" {
			t.Fatal("expected requestId in data")
		}
		operation, _ := data["operation"].(string)
		if operation != "write" {
			t.Fatalf("expected operation write, got %s", operation)
		}
		text, _ := data["text"].(string)
		if text != "hello clipboard" {
			t.Fatalf("expected text 'hello clipboard', got '%s'", text)
		}
		if !h.ResolveClipboardRequest(requestID, "") {
			t.Fatal("ResolveClipboardRequest returned false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clipboard_request event")
	}

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Errorf("WriteText() error = %v, want nil", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WriteText to complete")
	}
}

func TestBridgeClipboardHost_ReadTextSuccess(t *testing.T) {
	hub := sse.Global
	clientID := "test-clip-read-" + t.Name()
	client := hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)

	h := NewBridgeClipboardHost(hub)

	type result struct {
		text string
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		text, err := h.ReadText(context.Background())
		resultCh <- result{text: text, err: err}
	}()

	select {
	case msg := <-client.Events:
		event, _ := msg["event"].(string)
		if event != "clipboard_request" {
			t.Fatalf("expected event clipboard_request, got %s", event)
		}
		data, _ := msg["data"].(map[string]interface{})
		requestID, _ := data["requestId"].(string)
		operation, _ := data["operation"].(string)
		if operation != "read" {
			t.Fatalf("expected operation read, got %s", operation)
		}
		if !h.ResolveClipboardRequest(requestID, "clipboard content") {
			t.Fatal("ResolveClipboardRequest returned false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clipboard_request event")
	}

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Errorf("ReadText() error = %v, want nil", r.err)
		}
		if r.text != "clipboard content" {
			t.Errorf("ReadText() text = '%s', want 'clipboard content'", r.text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ReadText to complete")
	}
}

func TestBridgeClipboardHost_TextTooLarge(t *testing.T) {
	hub := sse.Global
	clientID := "test-clip-large-" + t.Name()
	_ = hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)

	h := NewBridgeClipboardHost(hub)
	largeText := strings.Repeat("x", maxClipboardTextSize+1)
	err := h.WriteText(context.Background(), largeText)
	if !errors.Is(err, ErrClipboardTextTooLarge) {
		t.Errorf("WriteText() error = %v, want ErrClipboardTextTooLarge", err)
	}
}

func TestBridgeClipboardHost_FailRequest(t *testing.T) {
	hub := sse.Global
	clientID := "test-clip-fail-" + t.Name()
	client := hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)

	h := NewBridgeClipboardHost(hub)
	testErr := errors.New("clipboard operation failed")

	type result struct{ err error }
	resultCh := make(chan result, 1)
	go func() {
		err := h.WriteText(context.Background(), "test")
		resultCh <- result{err: err}
	}()

	select {
	case msg := <-client.Events:
		data, _ := msg["data"].(map[string]interface{})
		requestID, _ := data["requestId"].(string)
		if !h.FailClipboardRequest(requestID, testErr) {
			t.Fatal("FailClipboardRequest returned false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clipboard_request event")
	}

	select {
	case r := <-resultCh:
		if r.err == nil {
			t.Error("WriteText() error = nil, want error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WriteText to complete")
	}
}

func TestBridgeClipboardHost_ContextCancel(t *testing.T) {
	hub := sse.Global
	clientID := "test-clip-cancel-" + t.Name()
	client := hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)

	h := NewBridgeClipboardHost(hub)
	ctx, cancel := context.WithCancel(context.Background())

	type result struct{ err error }
	resultCh := make(chan result, 1)
	go func() {
		err := h.WriteText(ctx, "test")
		resultCh <- result{err: err}
	}()

	select {
	case <-client.Events:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clipboard_request event")
	}

	cancel()

	select {
	case r := <-resultCh:
		if r.err == nil {
			t.Error("WriteText() error = nil, want context canceled error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for WriteText to complete")
	}
}

func TestBridgeClipboardHost_ResolveNotFound(t *testing.T) {
	h := NewBridgeClipboardHost(sse.Global)
	if h.ResolveClipboardRequest("nonexistent", "text") {
		t.Error("ResolveClipboardRequest returned true for non-existent request")
	}
	if h.FailClipboardRequest("nonexistent", errors.New("test")) {
		t.Error("FailClipboardRequest returned true for non-existent request")
	}
	if h.HasPendingRequest("nonexistent") {
		t.Error("HasPendingRequest returned true for non-existent request")
	}
}

func TestBridgeClipboardHost_IsAvailableWithClients(t *testing.T) {
	hub := sse.Global
	clientID := "test-clip-avail-" + t.Name()
	h := NewBridgeClipboardHost(hub)

	if h.IsAvailable() {
		t.Error("IsAvailable() = true before subscribe, want false")
	}

	_ = hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)

	if !h.IsAvailable() {
		t.Error("IsAvailable() = false after subscribe, want true")
	}
}
