package display

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type fakeDisplayCapability struct {
	displays []DisplayInfo
}

func (f *fakeDisplayCapability) FetchDisplays(ctx context.Context) ([]DisplayInfo, error) {
	return f.displays, nil
}

func (f *fakeDisplayCapability) FetchDisplay(ctx context.Context, displayID int) (DisplayInfo, error) {
	for _, d := range f.displays {
		if d.DisplayID == displayID {
			return d, nil
		}
	}
	return DisplayInfo{}, nil
}

func (f *fakeDisplayCapability) AddExists(displayID int) bool {
	for _, d := range f.displays {
		if d.DisplayID == displayID {
			return true
		}
	}
	return false
}

func (f *fakeDisplayCapability) NotifyDisplayAdded(displayID int)   {}
func (f *fakeDisplayCapability) NotifyDisplayRemoved(displayID int) {}
func (f *fakeDisplayCapability) NotifyDisplayChanged(displayID int) {}

func newTestHandler(displays []DisplayInfo) *DisplayService {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	listener := NewListener()
	topology := NewTopologyAdapter(false)
	policy := DefaultSelectionPolicy
	resolver := NewDefaultResolver(store, policy)
	cap := &fakeDisplayCapability{displays: displays}
	return NewDisplayService(store, classifier, listener, resolver, topology, policy, cap)
}

func TestHandler_List_Default(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true, Width: 1080, Height: 2400, DensityDPI: 420, State: string(DisplayStateOn), IsValid: true},
		{DisplayID: 1, Generation: 1, Name: "External", Width: 1920, Height: 1080, State: string(DisplayStateOn), IsValid: true},
	})

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationList,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	displays, ok := resp.Result["displays"].([]DisplayInfo)
	if !ok {
		t.Fatal("expected []DisplayInfo array")
	}
	if len(displays) != 2 {
		t.Errorf("expected 2 displays, got %d", len(displays))
	}
}

func TestHandler_List_Filtered(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true, Width: 1080, Height: 2400, State: string(DisplayStateOn), IsValid: true},
		{DisplayID: 1, Generation: 1, Name: "External", Width: 1920, Height: 1080, Presentation: true, State: string(DisplayStateOn), IsValid: true},
	})

	filterJSON := json.RawMessage(`{"presentationOnly": true}`)
	var payload map[string]any
	_ = json.Unmarshal(filterJSON, &payload)

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-filter-1",
		Operation:       OperationList,
		Payload:         payload,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	displays := resp.Result["displays"].([]DisplayInfo)
	if len(displays) != 1 {
		t.Errorf("expected 1 display after filter, got %d", len(displays))
	}
}

func TestHandler_Get_ByDisplayID(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true},
		{DisplayID: 2, Generation: 5, Name: "External"},
	})

	payload, _ := json.Marshal(map[string]any{"displayId": 2.0})
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-get-2",
		Operation:       OperationGet,
		Payload:         payloadMap,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	result, ok := resp.Result["display"].(DisplayInfo)
	if !ok {
		t.Fatalf("expected DisplayInfo, got %T: %v", resp.Result["display"], resp.Result["display"])
	}
	if result.DisplayID != 2 {
		t.Errorf("expected display 2, got %d", result.DisplayID)
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true},
	})

	payload, _ := json.Marshal(map[string]any{"displayId": 99.0})
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-get-404",
		Operation:       OperationGet,
		Payload:         payloadMap,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
	if resp.Error.Code != ErrDisplayNotFound {
		t.Errorf("expected DISPLAY_NOT_FOUND, got %s", resp.Error.Code)
	}
}

func TestHandler_Resolve_Found(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true, Width: 1080, Height: 2400},
	})

	payload, _ := json.Marshal(map[string]any{"displayId": 0.0})
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-resolve-ok",
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
		t.Errorf("expected displayId 0 in target, got %d", target.DisplayID)
	}
}

func TestHandler_Resolve_Ambiguous(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true},
		{DisplayID: 1, Generation: 1},
	})

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-resolve-amb",
		Operation:       OperationResolve,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
	if resp.Error.Code != ErrDisplayAmbiguous {
		t.Errorf("expected DISPLAY_AMBIGUOUS, got %s", resp.Error.Code)
	}
}

func TestHandler_UnknownOperation(t *testing.T) {
	svc := newTestHandler(nil)
	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-unknown",
		Operation:       "display.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Errorf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestHandler_Status_NoCapability(t *testing.T) {
	classifier := NewDisplayClassifier()
	store := NewDisplayStore(classifier)
	listener := NewListener()
	topology := NewTopologyAdapter(false)
	policy := DefaultSelectionPolicy
	resolver := NewDefaultResolver(store, policy)
	svc := NewDisplayService(store, classifier, listener, resolver, topology, policy, nil)

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-status-nocap",
		Operation:       OperationStatus,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result["supported"] != true {
		t.Error("expected supported=true")
	}
}

func TestHandler_Get_MissingParam(t *testing.T) {
	svc := newTestHandler([]DisplayInfo{
		{DisplayID: 0, Generation: 1, IsDefault: true},
	})

	resp := svc.Handle(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-get-empty",
		Operation:       OperationGet,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
}
