package shizuku

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockBridge struct {
	execute func(ctx context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error)
}

func (m *mockBridge) Execute(ctx context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	return m.execute(ctx, request)
}

func (m *mockBridge) Health(context.Context) androidnative.NativeBridgeHealth {
	return androidnative.NativeBridgeHealthReady
}

func TestHandlerForwardsExecute(t *testing.T) {
	bridge := &mockBridge{
		execute: func(ctx context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			if request.Operation != OperationExecute {
				t.Fatalf("operation = %s", request.Operation)
			}
			if request.Payload["executable"] != "input" {
				t.Fatalf("executable = %v", request.Payload["executable"])
			}
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "success",
				Result:          map[string]any{"exitCode": 0, "stdout": "ok"},
			}, nil
		},
	}
	resp := NewHandler(bridge).Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "request-1",
		Operation:       OperationExecute,
		Payload: map[string]any{
			"executable": "input",
			"args":       []string{"keyevent", "113"},
		},
	})

	if resp.Status != "success" {
		t.Fatalf("status = %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result["stdout"] != "ok" {
		t.Fatalf("result = %+v", resp.Result)
	}
}

func TestHandlerAcceptsCommandExecution(t *testing.T) {
	bridge := &mockBridge{
		execute: func(ctx context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			if request.Payload["command"] != "id" {
				t.Fatalf("command = %v", request.Payload["command"])
			}
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "success",
				Result:          map[string]any{"exitCode": 0, "stdout": "uid=2000"},
			}, nil
		},
	}
	resp := NewHandler(bridge).Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "request-command",
		Operation:       OperationExecute,
		Payload:         map[string]any{"command": "id"},
	})

	if resp.Status != "success" {
		t.Fatalf("status = %s: %+v", resp.Status, resp.Error)
	}
}

func TestHandlerRejectsAmbiguousExecute(t *testing.T) {
	bridge := &mockBridge{
		execute: func(context.Context, androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			t.Fatal("bridge must not be called")
			return androidnative.NativeBridgeResponse{}, nil
		},
	}
	resp := NewHandler(bridge).Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "request-ambiguous",
		Operation:       OperationExecute,
		Payload:         map[string]any{"command": "id", "executable": "id"},
	})

	if resp.Error == nil || resp.Error.DomainCode != "SHIZUKU_INVALID_REQUEST" {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestHandlerMapsPermissionRequired(t *testing.T) {
	bridge := &mockBridge{
		execute: func(ctx context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &androidnative.NativeBridgeError{
					Code:    "SHIZUKU_PERMISSION_REQUIRED",
					Message: "permission not granted",
				},
			}, nil
		},
	}
	resp := NewHandler(bridge).Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "request-2",
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	})

	if resp.Error == nil || resp.Error.Code != "USER_ACTION_REQUIRED" {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestHandlerMapsBridgeFailure(t *testing.T) {
	bridge := &mockBridge{
		execute: func(context.Context, androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{}, errors.New("relay closed")
		},
	}
	resp := NewHandler(bridge).Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "request-3",
		Operation:       OperationExecute,
		Payload:         map[string]any{"executable": "input"},
	})

	if resp.Error == nil || resp.Error.Code != "BRIDGE_DISCONNECTED" {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestHandlerForwardsExtendedOperations(t *testing.T) {
	operations := []string{
		OperationInfo,
		OperationPermissionCheck,
		OperationUserServiceStatus,
		OperationUserServiceBind,
		OperationUserServiceUnbind,
		OperationProcessStart,
		OperationProcessWrite,
		OperationProcessRead,
		OperationProcessWait,
		OperationProcessKill,
		OperationSystemService,
		OperationBinderTransact,
	}
	for _, operation := range operations {
		operation := operation
		bridge := &mockBridge{
			execute: func(ctx context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
				if request.Operation != operation {
					t.Fatalf("operation = %s, want %s", request.Operation, operation)
				}
				return androidnative.NativeBridgeResponse{
					ProtocolVersion: request.ProtocolVersion,
					RequestId:       request.RequestId,
					Status:          "success",
					Result:          map[string]any{"forwarded": true},
				}, nil
			},
		}
		resp := NewHandler(bridge).Execute(context.Background(), capability.AndroidBridgeRequest{
			ProtocolVersion: 1,
			RequestID:       "request-" + operation,
			Operation:       operation,
			Payload:         map[string]any{"probe": true},
		})
		if resp.Status != "success" || resp.Result["forwarded"] != true {
			t.Fatalf("%s response = %+v", operation, resp)
		}
	}
}
