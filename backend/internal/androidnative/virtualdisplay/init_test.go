package virtualdisplay

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockNativeBridge struct{}

func (m *mockNativeBridge) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
		Result: map[string]any{
			"displayId": 9999.0,
		},
	}, nil
}

func (m *mockNativeBridge) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	return androidnative.NativeBridgeHealthReady
}

func TestRegister(t *testing.T) {
	bridge := &mockNativeBridge{}
	adapter := androidnative.NewNativeProviderAdapter(bridge)
	svc := Register(adapter, bridge)
	if svc == nil {
		t.Fatal("Register returned nil service")
	}
	resp := adapter.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result["supported"] != true {
		t.Errorf("expected supported=true, got %v", resp.Result["supported"])
	}
}

func TestRegister_CreateAndGet(t *testing.T) {
	bridge := &mockNativeBridge{}
	adapter := androidnative.NewNativeProviderAdapter(bridge)
	Register(adapter, bridge)
	createResp := adapter.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-create",
		Operation:       OperationCreate,
		Payload: map[string]any{
			"width":      1080,
			"height":     1920,
			"densityDpi": 420,
		},
	})
	if createResp.Status != "success" {
		t.Fatalf("create failed: %s %+v", createResp.Status, createResp.Error)
	}
	displayMap, ok := createResp.Result["display"].(map[string]any)
	if !ok {
		t.Fatalf("expected display object, got %T: %v", createResp.Result["display"], createResp.Result["display"])
	}
	ref, _ := displayMap["ref"].(string)
	if ref == "" {
		t.Error("expected non-empty ref")
	}
	getResp := adapter.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-get",
		Operation:       OperationGet,
		Payload: map[string]any{
			"ref": ref,
		},
	})
	if getResp.Status != "success" {
		t.Fatalf("get failed: %s %+v", getResp.Status, getResp.Error)
	}
}

func TestRegister_Create_DuplicateRejected(t *testing.T) {
	bridge := &mockNativeBridge{}
	adapter := androidnative.NewNativeProviderAdapter(bridge)
	Register(adapter, bridge)
	adapter.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationCreate,
	})
	resp := adapter.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-2",
		Operation:       OperationCreate,
	})
	if resp.Status != "error" {
		t.Fatalf("expected error for duplicate create, got %s", resp.Status)
	}
}

func TestTools_BuildVirtualDisplayTools(t *testing.T) {
	tools := BuildVirtualDisplayTools()
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
	ids := map[string]bool{}
	for _, tool := range tools {
		ids[tool.ID] = true
		if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Errorf("tool %s: expected android_native runtime, got %s", tool.ID, tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID != RuntimeID {
			t.Errorf("tool %s: expected runtime ID %s, got %s", tool.ID, RuntimeID, tool.Runtime.RuntimeID)
		}
	}
	expectedIDs := []string{
		"android.virtual_display.status",
		"android.virtual_display.create",
		"android.virtual_display.get",
		"android.virtual_display.resize",
		"android.virtual_display.release",
	}
	for _, id := range expectedIDs {
		if !ids[id] {
			t.Errorf("missing tool: %s", id)
		}
	}
}

func TestTools_InputSchema(t *testing.T) {
	tools := BuildVirtualDisplayTools()
	for _, tool := range tools {
		if len(tool.InputSchema) == 0 {
			t.Errorf("tool %s: empty input schema", tool.ID)
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("tool %s: invalid input schema: %v", tool.ID, err)
		}
	}
}
