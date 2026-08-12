package virtualdisplay

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockVirtualBridge struct {
	execFunc func(ctx context.Context, op string, payload map[string]any) (map[string]any, error)
}

func (m *mockVirtualBridge) Execute(ctx context.Context, op string, payload map[string]any) (map[string]any, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, op, payload)
	}
	return map[string]any{"displayId": 9999.0}, nil
}

func TestHandler_UnknownOperation(t *testing.T) {
	svc := NewService(&Store{}, nil, DefaultPolicy(), &PrimaryResolver{})
	h := NewHandler(svc)
	resp := h.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "vd.unknown",
	})
	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Errorf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestHandler_Status_Success(t *testing.T) {
	store := &Store{}
	store.Insert(&VirtualDisplayRecord{
		DisplayID:  100,
		Width:      1080,
		Height:     1920,
		DensityDPI: 420,
		State:      StateReady,
	})
	svc := NewService(store, nil, DefaultPolicy(), &PrimaryResolver{})
	h := NewHandler(svc)
	resp := h.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %+v", resp.Status, resp.Error)
	}
	result, ok := resp.Result["active"].(bool)
	if !ok || !result {
		t.Errorf("expected active=true, got %v", resp.Result["active"])
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	svc := NewService(&Store{}, nil, DefaultPolicy(), &PrimaryResolver{})
	h := NewHandler(svc)
	resp := h.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationGet,
	})
	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
}

func TestHandler_Release_AlreadyReleased(t *testing.T) {
	svc := NewService(&Store{}, nil, DefaultPolicy(), &PrimaryResolver{})
	h := NewHandler(svc)
	resp := h.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationRelease,
		Payload: map[string]any{
			"ref": "vd_nonexistent",
		},
	})
	if resp.Status != "success" {
		t.Fatalf("expected success (idempotent), got %s: %+v", resp.Status, resp.Error)
	}
	released, _ := resp.Result["released"].(bool)
	if released {
		t.Error("expected released=false for nonexistent display")
	}
}

func TestHandler_Create_AlreadyExists(t *testing.T) {
	store := &Store{}
	store.Insert(&VirtualDisplayRecord{
		DisplayID:  100,
		State:      StateReady,
		Width:      1080,
		Height:     1920,
		DensityDPI: 420,
	})
	svc := NewService(store, &mockVirtualBridge{}, DefaultPolicy(), &PrimaryResolver{})
	h := NewHandler(svc)
	resp := h.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationCreate,
	})
	if resp.Status != "error" {
		t.Fatalf("expected error, got %s: %+v", resp.Status, resp.Error)
	}
}

func TestService_Create_And_Get(t *testing.T) {
	store := &Store{}
	svc := NewService(store, &mockVirtualBridge{}, DefaultPolicy(), &PrimaryResolver{})
	createResult, err := svc.Create(context.Background(), CreateRequest{
		Width:      1080,
		Height:     1920,
		DensityDPI: 420,
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if createResult.Display.DisplayID != 9999 {
		t.Errorf("expected displayID 9999, got %d", createResult.Display.DisplayID)
	}
	if createResult.Display.State != string(StateReady) {
		t.Errorf("expected state ready, got %s", createResult.Display.State)
	}
	getResult, err := svc.Get(context.Background(), GetRequest{})
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if getResult.Ref != createResult.Display.Ref {
		t.Errorf("ref mismatch: got %s, want %s", getResult.Ref, createResult.Display.Ref)
	}
}

func TestService_Create_Defaults(t *testing.T) {
	store := &Store{}
	svc := NewService(store, &mockVirtualBridge{}, DefaultPolicy(), &PrimaryResolver{})
	result, err := svc.Create(context.Background(), CreateRequest{})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if result.Display.Width != DefaultWidth {
		t.Errorf("expected default width %d, got %d", DefaultWidth, result.Display.Width)
	}
	if result.Display.Height != DefaultHeight {
		t.Errorf("expected default height %d, got %d", DefaultHeight, result.Display.Height)
	}
}

func TestService_Create_InvalidSize(t *testing.T) {
	store := &Store{}
	svc := NewService(store, &mockVirtualBridge{}, DefaultPolicy(), &PrimaryResolver{})
	_, err := svc.Create(context.Background(), CreateRequest{Width: 100, Height: 100})
	if err == nil {
		t.Error("expected error for invalid size")
	}
}

func TestService_Resize(t *testing.T) {
	store := &Store{}
	store.Insert(&VirtualDisplayRecord{
		DisplayID:  100,
		Width:      1080,
		Height:     1920,
		DensityDPI: 420,
		State:      StateReady,
	})
	rec := store.Get()
	svc := NewService(store, &mockVirtualBridge{}, DefaultPolicy(), &PrimaryResolver{})
	result, err := svc.Resize(context.Background(), ResizeRequest{
		Ref:    rec.Ref,
		Width:  720,
		Height: 1280,
	})
	if err != nil {
		t.Fatalf("resize error: %v", err)
	}
	if result.Generation != rec.Generation+1 {
		t.Errorf("expected generation bump, got %d", result.Generation)
	}
}

func TestService_Release(t *testing.T) {
	store := &Store{}
	store.Insert(&VirtualDisplayRecord{
		DisplayID:  100,
		Width:      1080,
		Height:     1920,
		DensityDPI: 420,
		State:      StateReady,
	})
	rec := store.Get()
	svc := NewService(store, &mockVirtualBridge{}, DefaultPolicy(), &PrimaryResolver{})
	result, err := svc.Release(context.Background(), ReleaseRequest{Ref: rec.Ref})
	if err != nil {
		t.Fatalf("release error: %v", err)
	}
	if !result.Released {
		t.Error("expected released=true")
	}
	if !result.WasActive {
		t.Error("expected wasActive=true")
	}
}

func TestState_IsActive(t *testing.T) {
	cases := map[State]bool{
		StateCreating:  true,
		StateReady:     true,
		StatePaused:    true,
		StateResizing:  true,
		StateReleasing: false,
		StateReleased:  false,
		StateFailed:    false,
	}
	for state, want := range cases {
		if state.IsActive() != want {
			t.Errorf("State(%s).IsActive() = %v, want %v", state, state.IsActive(), want)
		}
	}
}

func TestState_IsTerminal(t *testing.T) {
	cases := map[State]bool{
		StateCreating:  false,
		StateReady:     false,
		StateReleasing: false,
		StateReleased:  true,
		StateFailed:    true,
	}
	for state, want := range cases {
		if state.IsTerminal() != want {
			t.Errorf("State(%s).IsTerminal() = %v, want %v", state, state.IsTerminal(), want)
		}
	}
}
