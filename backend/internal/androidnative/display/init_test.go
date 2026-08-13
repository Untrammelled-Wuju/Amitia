package display

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockDisplayCapability struct {
	displays []DisplayInfo
}

func (m *mockDisplayCapability) FetchDisplays(ctx context.Context) ([]DisplayInfo, error) {
	if m.displays != nil {
		return m.displays, nil
	}
	return []DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true, Width: 1080, Height: 2400, DensityDPI: 420, State: string(DisplayStateOn), IsValid: true, Name: "Built-in"},
	}, nil
}

func (m *mockDisplayCapability) FetchDisplay(ctx context.Context, displayID int) (DisplayInfo, error) {
	for _, d := range m.displays {
		if d.DisplayID == displayID {
			return d, nil
		}
	}
	return DisplayInfo{}, nil
}

func (m *mockDisplayCapability) AddExists(displayID int) bool {
	for _, d := range m.displays {
		if d.DisplayID == displayID {
			return true
		}
	}
	return false
}

func (m *mockDisplayCapability) NotifyDisplayAdded(displayID int)    {}
func (m *mockDisplayCapability) NotifyDisplayRemoved(displayID int)  {}
func (m *mockDisplayCapability) NotifyDisplayChanged(displayID int)  {}

type mockNativeBridgeDisplay struct{}

func (m *mockNativeBridgeDisplay) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
		Result:          map[string]any{},
	}, nil
}

func (m *mockNativeBridgeDisplay) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	return androidnative.NativeBridgeHealthReady
}

func TestRegister(t *testing.T) {
	bridge := &mockNativeBridgeDisplay{}
	provider := androidnative.NewProvider(bridge)
	cap := &mockDisplayCapability{}
	svc, err := Register(provider, bridge, cap)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if svc == nil {
		t.Fatal("Register returned nil service")
	}
}

func TestRegister_ListOperation(t *testing.T) {
	bridge := &mockNativeBridgeDisplay{}
	provider := androidnative.NewProvider(bridge)
	cap := &mockDisplayCapability{}
	_, err := Register(provider, bridge, cap)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	resp := provider.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-list-1",
		Operation:       OperationList,
	})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	displays, ok := resp.Result["displays"].([]DisplayInfo)
	if !ok {
		t.Fatalf("expected []DisplayInfo, got %T", resp.Result["displays"])
	}
	if len(displays) != 1 {
		t.Errorf("expected 1 display, got %d", len(displays))
	}
}

func TestRegister_GetOperation(t *testing.T) {
	bridge := &mockNativeBridgeDisplay{}
	provider := androidnative.NewProvider(bridge)
	cap := &mockDisplayCapability{}
	_, err := Register(provider, bridge, cap)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{"displayId": 0.0})
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	resp := provider.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-get-1",
		Operation:       OperationGet,
		Payload:         payloadMap,
	})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
}

func TestRegister_GetNotFound(t *testing.T) {
	bridge := &mockNativeBridgeDisplay{}
	provider := androidnative.NewProvider(bridge)
	cap := &mockDisplayCapability{}
	_, err := Register(provider, bridge, cap)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{"displayId": 99.0})
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	resp := provider.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-get-404",
		Operation:       OperationGet,
		Payload:         payloadMap,
	})
	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
}

func TestRegister_StatusOperation(t *testing.T) {
	bridge := &mockNativeBridgeDisplay{}
	provider := androidnative.NewProvider(bridge)
	cap := &mockDisplayCapability{}
	_, err := Register(provider, bridge, cap)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	resp := provider.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-status-1",
		Operation:       OperationStatus,
	})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result["supported"] != true {
		t.Errorf("expected supported=true, got %v", resp.Result["supported"])
	}
}

func TestRegister_ResolveOperation(t *testing.T) {
	bridge := &mockNativeBridgeDisplay{}
	provider := androidnative.NewProvider(bridge)
	cap := &mockDisplayCapability{}
	_, err := Register(provider, bridge, cap)
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{"displayId": 0.0})
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	resp := provider.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-resolve-1",
		Operation:       OperationResolve,
		Payload:         payloadMap,
	})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	target, ok := resp.Result["target"].(DisplayTarget)
	if !ok {
		t.Fatalf("expected DisplayTarget, got %T: %v", resp.Result["target"], resp.Result["target"])
	}
	if target.DisplayID != 0 {
		t.Errorf("expected displayId 0, got %d", target.DisplayID)
	}
}

func TestBuildDisplayTools(t *testing.T) {
	tools := BuildDisplayTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	ids := map[string]bool{}
	for _, tool := range tools {
		ids[tool.ID] = true
		if tool.Runtime.RuntimeType != "android_native" {
			t.Errorf("tool %s: expected android_native runtime, got %s", tool.ID, tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID != RuntimeID {
			t.Errorf("tool %s: expected runtime ID %s, got %s", tool.ID, RuntimeID, tool.Runtime.RuntimeID)
		}
	}
	expectedIDs := []string{
		"android.display.status",
		"android.display.list",
		"android.display.get",
	}
	for _, id := range expectedIDs {
		if !ids[id] {
			t.Errorf("missing tool: %s", id)
		}
	}
}

func TestBuildDisplayTools_InputSchema(t *testing.T) {
	tools := BuildDisplayTools()
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
