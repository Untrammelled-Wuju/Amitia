package root

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockRootBridge struct {
	executeFunc func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error)
	healthFunc  func(ctx context.Context) androidnative.NativeBridgeHealth
}

func (m *mockRootBridge) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockRootBridge) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return androidnative.NativeBridgeHealthReady
}

func TestRootHandler_UnknownOperation(t *testing.T) {
	bridge := &mockRootBridge{}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "root.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestRootHandler_Status_NilBridge(t *testing.T) {
	handler := NewRootHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_BRIDGE_UNAVAILABLE {
		t.Fatalf("expected ROOT_BRIDGE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestRootHandler_Status_Success(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"platformSupported":   true,
					"rootFramework":       "Magisk",
					"rootManagerDetected": true,
					"suBinaryDetected":    true,
					"authorizationState":  "granted",
					"rootAvailable":       true,
					"backend":             "su",
					"state":               "authorized",
				},
			}, nil
		},
	}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["state"] != "authorized" {
		t.Fatalf("expected authorized state, got %v", resp.Result["state"])
	}
}

func TestRootHandler_Request_NilBridge(t *testing.T) {
	handler := NewRootHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationRequest,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_BRIDGE_UNAVAILABLE {
		t.Fatalf("expected ROOT_BRIDGE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestRootHandler_Request_UserActionRequired(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "error",
				Error: &androidnative.NativeBridgeError{
					Code:       "USER_ACTION_REQUIRED",
					Message:    "waiting for user authorization",
					DomainCode: ROOT_REQUEST_FAILED,
				},
			}, nil
		},
	}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationRequest,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "USER_ACTION_REQUIRED" {
		t.Fatalf("expected USER_ACTION_REQUIRED, got %+v", resp.Error)
	}
}

func TestRootHandler_Execute_NilBridge(t *testing.T) {
	handler := NewRootHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationExecute,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_BRIDGE_UNAVAILABLE {
		t.Fatalf("expected ROOT_BRIDGE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestRootHandler_Execute_EmptyPayload(t *testing.T) {
	bridge := &mockRootBridge{}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationExecute,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %+v", resp.Error)
	}
}

func TestRootHandler_Execute_EmptyExecutable(t *testing.T) {
	bridge := &mockRootBridge{}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationExecute,
		Payload: map[string]any{
			"executable": "",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %+v", resp.Error)
	}
}

func TestRootHandler_Execute_ShellExecutableNoMode(t *testing.T) {
	bridge := &mockRootBridge{}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationExecute,
		Payload: map[string]any{
			"executable": "sh",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_COMMAND_NOT_ALLOWED {
		t.Fatalf("expected ROOT_COMMAND_NOT_ALLOWED, got %+v", resp.Error)
	}
}

func TestRootHandler_Execute_Success(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"exitCode":          0,
					"exitCodeAvailable": true,
					"stdout":            "uid=0(root) gid=0(root)",
					"stderr":            "",
					"durationMs":        12,
					"timedOut":          false,
				},
			}, nil
		},
	}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationExecute,
		Payload: map[string]any{
			"executable": "id",
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["stdout"] != "uid=0(root) gid=0(root)" {
		t.Fatalf("expected uid=0 output, got %v", resp.Result["stdout"])
	}
}

func TestRootHandler_Execute_WithTimeout(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"exitCode":          0,
					"exitCodeAvailable": true,
					"stdout":            "",
					"stderr":            "",
					"durationMs":        5,
					"timedOut":          true,
				},
			}, nil
		},
	}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationExecute,
		Payload: map[string]any{
			"executable": "sleep",
			"args":       []string{"10"},
			"timeoutMs":  1000,
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["timedOut"] != true {
		t.Fatalf("expected timedOut true, got %v", resp.Result["timedOut"])
	}
}

func TestRootHandler_RequestIDMismatch(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       "wrong-id",
				Status:          "success",
			}, nil
		},
	}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_INVALID_RESPONSE {
		t.Fatalf("expected ROOT_INVALID_RESPONSE, got %+v", resp.Error)
	}
}

func TestRootHandler_BridgeError(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{}, context.DeadlineExceeded
		},
	}
	handler := NewRootHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != ROOT_BRIDGE_UNAVAILABLE {
		t.Fatalf("expected ROOT_BRIDGE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestInternalRootExecutor(t *testing.T) {
	bridge := &mockRootBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"exitCode":          0,
					"exitCodeAvailable": true,
					"stdout":            "uid=0(root)",
					"stderr":            "",
					"durationMs":        5,
					"timedOut":          false,
				},
			}, nil
		},
	}
	handler := NewRootHandler(bridge)

	executor := handler.InternalExecutor()
	result, err := executor.ExecuteRoot(context.Background(), ExecuteRequest{
		Executable: "id",
	}, InternalExecuteOptions{
		Timeout: 5000,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "uid=0(root)" {
		t.Fatalf("expected uid=0(root), got %s", result.Stdout)
	}
}
