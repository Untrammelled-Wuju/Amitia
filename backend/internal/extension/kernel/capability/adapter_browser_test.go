package capability

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestBrowserRuntimeAdapterSupports(t *testing.T) {
	adapter := NewBrowserRuntimeAdapter(nil, nil)

	if !adapter.Supports(RuntimeBinding{RuntimeType: RuntimeTypeBrowser}) {
		t.Fatal("adapter should support RuntimeTypeBrowser")
	}
	if adapter.Supports(RuntimeBinding{RuntimeType: RuntimeTypeBuiltin}) {
		t.Fatal("adapter should not support RuntimeTypeBuiltin")
	}
}

func TestBrowserRuntimeAdapterExecuteNoCaller(t *testing.T) {
	adapter := NewBrowserRuntimeAdapter(nil, nil)
	result := adapter.Execute(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeBrowser}, ToolInvocationContext{InvocationID: "test"}, nil)
	if result.Status != ToolResultStatusFailed {
		t.Fatalf("expected failed status, got: %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != ErrorCodeRuntimeUnavailable {
		t.Fatalf("expected runtime_unavailable error, got: %v", result.Error)
	}
}

func TestBrowserRuntimeAdapterExecuteSuccess(t *testing.T) {
	called := false
	call := func(_ context.Context, handlerName string, _ ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
		called = true
		if handlerName != "test_handler" {
			t.Fatalf("unexpected handler name: %s", handlerName)
		}
		return json.RawMessage(`{"result":"ok"}`), nil
	}

	adapter := NewBrowserRuntimeAdapter(call, nil)
	result := adapter.Execute(
		context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeBrowser, HandlerName: "test_handler"},
		ToolInvocationContext{InvocationID: "test-1"},
		json.RawMessage(`{"key":"value"}`),
	)

	if !called {
		t.Fatal("caller should have been called")
	}
	if result.Status != ToolResultStatusSuccess {
		t.Fatalf("expected success status, got: %s", result.Status)
	}
	if result.InvocationID != "test-1" {
		t.Fatalf("unexpected invocation ID: %s", result.InvocationID)
	}
}

func TestBrowserRuntimeAdapterExecuteError(t *testing.T) {
	call := func(_ context.Context, _ string, _ ToolInvocationContext, _ json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("browser execution failed")
	}

	adapter := NewBrowserRuntimeAdapter(call, nil)
	result := adapter.Execute(
		context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeBrowser, HandlerName: "navigate"},
		ToolInvocationContext{InvocationID: "test-2"},
		nil,
	)

	if result.Status != ToolResultStatusFailed {
		t.Fatalf("expected failed status, got: %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be set")
	}
}

func TestBrowserRuntimeAdapterHealth(t *testing.T) {
	healthCalled := false
	health := func(_ context.Context) HealthStatus {
		healthCalled = true
		return HealthReady
	}

	adapter := NewBrowserRuntimeAdapter(nil, health)
	status := adapter.Health(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeBrowser})

	if !healthCalled {
		t.Fatal("health function should have been called")
	}
	if status != HealthReady {
		t.Fatalf("expected ready health, got: %s", status)
	}
}

func TestBrowserRuntimeAdapterHealthNoFunc(t *testing.T) {
	adapter := NewBrowserRuntimeAdapter(nil, nil)
	status := adapter.Health(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeBrowser})
	if status != HealthUnknown {
		t.Fatalf("expected unknown health, got: %s", status)
	}
}
