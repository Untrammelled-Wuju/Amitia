package adb

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestADBHandler_UnknownOperation(t *testing.T) {
	handler := NewADBHandler(&ADBConfig{Enabled: false})

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-test-1",
		Operation:       "adb.root",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestADBHandler_Status_Disabled(t *testing.T) {
	handler := NewADBHandler(&ADBConfig{Enabled: false})

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-status-1",
		Operation:       "adb.status",
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
	state, ok := resp.Result["state"].(string)
	if !ok || state != BackendUnavailable {
		t.Fatalf("expected state=unavailable, got %v", resp.Result["state"])
	}
	if resp.Result["supported"] != false {
		t.Fatalf("expected supported=false, got %v", resp.Result["supported"])
	}
}

func TestADBHandler_Execute_EmptyPayload(t *testing.T) {
	handler := NewADBHandler(&ADBConfig{Enabled: true})

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-exec-empty",
		Operation:       "adb.execute",
		Payload:         nil,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
}

func TestADBHandler_Execute_Disabled(t *testing.T) {
	handler := NewADBHandler(&ADBConfig{Enabled: false})

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-exec-disabled",
		Operation:       "adb.execute",
		Payload: map[string]any{
			"executable": "getprop",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
}

func TestADBHandler_Execute_NotAllowedCommand(t *testing.T) {
	handler := NewADBHandler(&ADBConfig{
		Enabled:        true,
		ExecutablePath: "/fakeroot/adb",
		Timeout:        5 * time.Second,
	})

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-exec-forbidden",
		Operation:       "adb.execute",
		Payload: map[string]any{
			"executable": "ls",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
}

func TestMapPolicyErrorToAndroidError(t *testing.T) {
	tests := map[string]string{
		ADB_DEVICE_UNAUTHORIZED: "USER_ACTION_REQUIRED",
		ADB_COMMAND_NOT_ALLOWED: "AUTHORIZATION_DENIED",
		ADB_INVALID_ARGUMENT:    "AUTHORIZATION_DENIED",
		ADB_NO_DEVICE:           "PLATFORM_NOT_SUPPORTED",
		ADB_DEVICE_AMBIGUOUS:    "PROVIDER_UNAVAILABLE",
		ADB_TIMEOUT:             "BRIDGE_TIMEOUT",
		ADB_CANCELLED:           "CANCELLED",
		ADB_DEVICE_OFFLINE:      "PROVIDER_UNAVAILABLE",
		ADB_EXECUTION_FAILED:    "PROVIDER_UNAVAILABLE",
	}

	for code, expected := range tests {
		policyErr := &PolicyError{Code: code, Message: "test"}
		actual := mapPolicyErrorToAndroidError(policyErr)
		if actual != expected {
			t.Errorf("mapPolicyError(%s) = %s, expected %s", code, actual, expected)
		}
	}
}

func TestValidateExecuteRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *ADBExecuteRequest
		wantErr bool
	}{
		{
			name: "empty executable",
			req:  &ADBExecuteRequest{},
			wantErr: true,
		},
		{
			name: "shell executable rejected",
			req:  &ADBExecuteRequest{Executable: "sh"},
			wantErr: true,
		},
		{
			name: "bash executable rejected",
			req:  &ADBExecuteRequest{Executable: "bash"},
			wantErr: true,
		},
		{
			name: "too many args",
			req: &ADBExecuteRequest{
				Executable: "getprop",
				Args:       make([]string, maxArgCount+1),
			},
			wantErr: true,
		},
		{
			name: "stdin too large",
			req: &ADBExecuteRequest{
				Executable: "getprop",
				Stdin:      string(make([]byte, maxInputBytes+1)),
			},
			wantErr: true,
		},
		{
			name: "valid getprop",
			req: &ADBExecuteRequest{
				Executable: "getprop",
				Args:       []string{"ro.build.version.sdk"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExecuteRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExecuteRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestADBConfig_Defaults(t *testing.T) {
	config := &ADBConfig{
		Enabled:        true,
		Backend:        "cli",
		ExecutablePath: "/usr/bin/adb",
		Timeout:        10 * time.Second,
	}
	if config.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", config.Timeout)
	}
}

func TestInternalADBExecutor_Interface(t *testing.T) {
	var _ InternalADBExecutor = (*ADBHandler)(nil)
}

func TestAndroidNativeProviderUnavailable(t *testing.T) {
	if androidnative.PROVIDER_UNAVAILABLE == "" {
		t.Errorf("PROVIDER_UNAVAILABLE should not be empty")
	}
}
