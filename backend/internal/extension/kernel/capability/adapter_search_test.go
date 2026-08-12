package capability

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestSearchRuntimeAdapter_Supports_Search(t *testing.T) {
	a := NewSearchRuntimeAdapter(nil, nil)
	if !a.Supports(RuntimeBinding{RuntimeType: RuntimeTypeSearch}) {
		t.Fatal("should support search runtime type")
	}
}

func TestSearchRuntimeAdapter_Supports_RejectOther(t *testing.T) {
	a := NewSearchRuntimeAdapter(nil, nil)
	if a.Supports(RuntimeBinding{RuntimeType: RuntimeTypeBuiltin}) {
		t.Fatal("should not support builtin runtime type")
	}
	if a.Supports(RuntimeBinding{RuntimeType: RuntimeTypeMCP}) {
		t.Fatal("should not support mcp runtime type")
	}
}

func TestSearchRuntimeAdapter_Execute_NilCall(t *testing.T) {
	a := NewSearchRuntimeAdapter(nil, nil)
	inv := ToolInvocationContext{InvocationID: "i1", UserID: "u1"}
	res := a.Execute(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeSearch}, inv, json.RawMessage(`{}`))
	if res.Status != ToolResultStatusFailed {
		t.Fatalf("expected failed, got %s", res.Status)
	}
	if res.Error == nil || res.Error.Code != ErrorCodeExecutionFailed {
		t.Fatalf("expected execution failed error, got %v", res.Error)
	}
	if res.InvocationID != "i1" {
		t.Fatalf("invocation id not propagated: %s", res.InvocationID)
	}
}

func TestSearchRuntimeAdapter_Execute_Success(t *testing.T) {
	expected := json.RawMessage(`{"query":"test","results":[]}`)
	a := NewSearchRuntimeAdapter(func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) (json.RawMessage, error) {
		if providerID != "myprovider" {
			t.Fatalf("providerID mismatch: %s", providerID)
		}
		if handlerName != "search.general" {
			t.Fatalf("handlerName mismatch: %s", handlerName)
		}
		if string(input) != `{"query":"test"}` {
			t.Fatalf("input mismatch: %s", input)
		}
		return expected, nil
	}, nil)
	inv := ToolInvocationContext{InvocationID: "i2", UserID: "u1"}
	res := a.Execute(context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeSearch, RuntimeID: "myprovider", HandlerName: "search.general"},
		inv, json.RawMessage(`{"query":"test"}`))
	if res.Status != ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s (err: %v)", res.Status, res.Error)
	}
	if string(res.Structured) != string(expected) {
		t.Fatalf("output mismatch: %s", res.Structured)
	}
}

func TestSearchRuntimeAdapter_Execute_DefaultProviderID(t *testing.T) {
	a := NewSearchRuntimeAdapter(func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) (json.RawMessage, error) {
		if providerID != "default" {
			t.Fatalf("expected 'default' providerID, got %s", providerID)
		}
		return json.RawMessage(`{}`), nil
	}, nil)
	inv := ToolInvocationContext{InvocationID: "i3", UserID: "u1"}
	a.Execute(context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeSearch},
		inv, json.RawMessage(`{}`))
}

func TestSearchRuntimeAdapter_Execute_ToolError(t *testing.T) {
	a := NewSearchRuntimeAdapter(func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) (json.RawMessage, error) {
		return nil, &ToolError{Code: ErrorCodePermissionDenied, Message: "no network", UserVisible: true}
	}, nil)
	inv := ToolInvocationContext{InvocationID: "i4", UserID: "u1"}
	res := a.Execute(context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeSearch},
		inv, json.RawMessage(`{}`))
	if res.Status != ToolResultStatusFailed {
		t.Fatal("expected failed")
	}
	if res.Error == nil || res.Error.Code != ErrorCodePermissionDenied {
		t.Fatalf("tool error not propagated: %v", res.Error)
	}
}

func TestSearchRuntimeAdapter_Execute_WrapGenericError(t *testing.T) {
	a := NewSearchRuntimeAdapter(func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) (json.RawMessage, error) {
		return nil, errors.New("network down")
	}, nil)
	inv := ToolInvocationContext{InvocationID: "i5", UserID: "u1"}
	res := a.Execute(context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeSearch},
		inv, json.RawMessage(`{}`))
	if res.Status != ToolResultStatusFailed {
		t.Fatal("expected failed")
	}
	if res.Error == nil || res.Error.Code != ErrorCodeExecutionFailed {
		t.Fatalf("not wrapped as execution failed: %v", res.Error)
	}
}

func TestSearchRuntimeAdapter_Health_Nil(t *testing.T) {
	a := NewSearchRuntimeAdapter(nil, nil)
	s := a.Health(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeSearch})
	if s != HealthUnknown {
		t.Fatalf("expected unknown, got %s", s)
	}
}

func TestSearchRuntimeAdapter_Health_Binding(t *testing.T) {
	a := NewSearchRuntimeAdapter(nil, func(ctx context.Context, providerID string) HealthStatus {
		if providerID != "brave" {
			t.Fatalf("wrong provider for health: %s", providerID)
		}
		return HealthReady
	})
	s := a.Health(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeSearch, RuntimeID: "brave"})
	if s != HealthReady {
		t.Fatalf("expected ready, got %s", s)
	}
}

func TestSearchRuntimeAdapter_Health_DefaultProvider(t *testing.T) {
	a := NewSearchRuntimeAdapter(nil, func(ctx context.Context, providerID string) HealthStatus {
		if providerID != "default" {
			t.Fatalf("expected 'default', got %s", providerID)
		}
		return HealthDegraded
	})
	s := a.Health(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeSearch})
	if s != HealthDegraded {
		t.Fatalf("expected degraded, got %s", s)
	}
}
