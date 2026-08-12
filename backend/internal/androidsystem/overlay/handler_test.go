package overlay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/androidsystem"
)

type fakeOverlayClient struct {
	statusResult    CapabilityState
	statusErr       error
	permResult      PermissionResult
	permErr         error
	createResult    OverlayInstance
	createErr       error
	updateResult    OverlayInstance
	updateErr       error
	showResult      OverlayInstance
	showErr         error
	hideResult      OverlayInstance
	hideErr         error
	closeErr        error
	closeAllCount   int
	closeAllErr     error
	listResult      []OverlayInstance
	listErr         error
}

func (f *fakeOverlayClient) Status(ctx context.Context) (CapabilityState, error) {
	return f.statusResult, f.statusErr
}

func (f *fakeOverlayClient) RequestPermission(ctx context.Context) (PermissionResult, error) {
	return f.permResult, f.permErr
}

func (f *fakeOverlayClient) Create(ctx context.Context, req CreateRequest) (OverlayInstance, error) {
	return f.createResult, f.createErr
}

func (f *fakeOverlayClient) Update(ctx context.Context, req UpdateRequest) (OverlayInstance, error) {
	return f.updateResult, f.updateErr
}

func (f *fakeOverlayClient) Show(ctx context.Context, overlayID string) (OverlayInstance, error) {
	return f.showResult, f.showErr
}

func (f *fakeOverlayClient) Hide(ctx context.Context, overlayID string) (OverlayInstance, error) {
	return f.hideResult, f.hideErr
}

func (f *fakeOverlayClient) Close(ctx context.Context, overlayID string) error {
	return f.closeErr
}

func (f *fakeOverlayClient) List(ctx context.Context) ([]OverlayInstance, error) {
	return f.listResult, f.listErr
}

func (f *fakeOverlayClient) CloseAll(ctx context.Context) (int, error) {
	return f.closeAllCount, f.closeAllErr
}

func newDefaultFakeClient() *fakeOverlayClient {
	return &fakeOverlayClient{
		statusResult: CapabilityState{
			Supported:         true,
			PermissionGranted: true,
			NativeHostReady:   true,
			CanCreate:         true,
			State:             StateAvailable,
		},
		listResult: []OverlayInstance{},
	}
}

func TestHandlerStatus(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s", resp.Status)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
	if resp.Result["supported"] != true {
		t.Errorf("expected supported=true, got %v", resp.Result["supported"])
	}
}

func TestHandlerStatusError(t *testing.T) {
	client := newDefaultFakeClient()
	client.statusErr = newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, "host not ready")
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != OVERLAY_NATIVE_HOST_UNAVAILABLE {
		t.Errorf("expected OVERLAY_NATIVE_HOST_UNAVAILABLE, got %v", resp.Error)
	}
}

func TestHandlerPermissionRequest(t *testing.T) {
	client := newDefaultFakeClient()
	client.permResult = PermissionResult{
		Opened:           true,
		UserActionRequired: true,
		PermissionGranted: false,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationPermissionRequest,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s", resp.Status)
	}
	if resp.Result["opened"] != true {
		t.Errorf("expected opened=true, got %v", resp.Result["opened"])
	}
}

func TestHandlerCreate(t *testing.T) {
	client := newDefaultFakeClient()
	client.createResult = OverlayInstance{
		ID:      "ovl_test123",
		Kind:    OverlayKindText,
		Visible: true,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationCreate,
		Payload: map[string]any{
			"kind": OverlayKindText,
			"content": map[string]any{
				"text": "Hello World",
			},
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	overlay, ok := resp.Result["overlay"].(OverlayInstance)
	if !ok {
		t.Errorf("expected overlay in result, got %v", resp.Result)
	}
	if overlay.ID != "ovl_test123" {
		t.Errorf("expected overlay ID ovl_test123, got %s", overlay.ID)
	}
}

func TestHandlerCreatePermissionRequired(t *testing.T) {
	client := newDefaultFakeClient()
	client.statusResult.PermissionGranted = false
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationCreate,
		Payload: map[string]any{
			"kind": OverlayKindText,
		},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != OVERLAY_PERMISSION_REQUIRED {
		t.Errorf("expected OVERLAY_PERMISSION_REQUIRED, got %v", resp.Error)
	}
}

func TestHandlerCreateInvalidKind(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationCreate,
		Payload: map[string]any{
			"kind": "invalid_kind",
		},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != OVERLAY_INVALID_KIND {
		t.Errorf("expected OVERLAY_INVALID_KIND, got %v", resp.Error)
	}
}

func TestHandlerUpdate(t *testing.T) {
	client := newDefaultFakeClient()
	client.updateResult = OverlayInstance{
		ID:      "ovl_test123",
		Kind:    OverlayKindText,
		Visible: true,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationUpdate,
		Payload: map[string]any{
			"overlayId": "ovl_test123",
			"content": map[string]any{
				"text": "Updated content",
			},
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerUpdateMissingID(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationUpdate,
		Payload:   map[string]any{},
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != OVERLAY_INVALID_INPUT {
		t.Errorf("expected OVERLAY_INVALID_INPUT, got %v", resp.Error)
	}
}

func TestHandlerShow(t *testing.T) {
	client := newDefaultFakeClient()
	client.showResult = OverlayInstance{
		ID:      "ovl_test123",
		Visible: true,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationShow,
		Payload: map[string]any{
			"overlayId": "ovl_test123",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerHide(t *testing.T) {
	client := newDefaultFakeClient()
	client.hideResult = OverlayInstance{
		ID:      "ovl_test123",
		Visible: false,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationHide,
		Payload: map[string]any{
			"overlayId": "ovl_test123",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
}

func TestHandlerClose(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationClose,
		Payload: map[string]any{
			"overlayId": "ovl_test123",
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["closed"] != true {
		t.Errorf("expected closed=true, got %v", resp.Result["closed"])
	}
}

func TestHandlerList(t *testing.T) {
	client := newDefaultFakeClient()
	client.listResult = []OverlayInstance{
		{ID: "ovl_1", Kind: OverlayKindText},
		{ID: "ovl_2", Kind: OverlayKindCard},
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationList,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["count"] != 2 {
		t.Errorf("expected count=2, got %v", resp.Result["count"])
	}
}

func TestHandlerCloseAll(t *testing.T) {
	client := newDefaultFakeClient()
	client.closeAllCount = 3
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationCloseAll,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["closedCount"] != 3 {
		t.Errorf("expected closedCount=3, got %v", resp.Result["closedCount"])
	}
}

func TestHandlerUnknownOperation(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: "unknown.op",
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != OVERLAY_UNSUPPORTED {
		t.Errorf("expected OVERLAY_UNSUPPORTED, got %v", resp.Error)
	}
}

func TestHandlerGenericError(t *testing.T) {
	client := newDefaultFakeClient()
	client.statusErr = errors.New("generic error")
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != OVERLAY_NATIVE_HOST_UNAVAILABLE {
		t.Errorf("expected OVERLAY_NATIVE_HOST_UNAVAILABLE, got %v", resp.Error)
	}
}

func TestHandlerCreateRequestParsing(t *testing.T) {
	req := CreateRequest{
		Kind: OverlayKindText,
		Content: map[string]any{
			"text": "test",
		},
	}
	x, y, w, h := 10, 20, 100, 200
	focusable, touchable, draggable := true, true, false
	ttl := int64(60000)
	req.X = &x
	req.Y = &y
	req.Width = &w
	req.Height = &h
	req.Focusable = &focusable
	req.Touchable = &touchable
	req.Draggable = &draggable
	req.TTLms = &ttl
	req.Gravity = GravityTopLeft

	if req.Kind != OverlayKindText {
		t.Errorf("expected kind=text, got %s", req.Kind)
	}
	if *req.X != 10 {
		t.Errorf("expected x=10, got %d", *req.X)
	}
	if *req.TTLms != 60000 {
		t.Errorf("expected ttl=60000, got %d", *req.TTLms)
	}
}

func TestBlockedClient(t *testing.T) {
	client := NewBlockedOverlayClient()
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationStatus,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
}

func TestHandlerCloseReleasesClient(t *testing.T) {
	client := newDefaultFakeClient()
	handler := NewOverlayHandler(client)

	handler.Close()

	status, err := client.Status(context.Background())
	if err != nil && status.State != StateUnsupported {
		t.Logf("client status after close: %v, err: %v", status, err)
	}
}

func TestCreateRequestZeroValues(t *testing.T) {
	req := CreateRequest{}

	if req.Kind != "" {
		t.Errorf("expected empty kind, got %s", req.Kind)
	}
	if req.X != nil {
		t.Errorf("expected nil x")
	}
	if req.Content != nil {
		t.Errorf("expected nil content")
	}
}

func TestHandlerWithNilClient(t *testing.T) {
	handler := NewOverlayHandler(nil)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationList,
	})

	if resp.Status != "error" {
		t.Errorf("expected error, got %s", resp.Status)
	}
}

func TestHandlerListEmpty(t *testing.T) {
	client := newDefaultFakeClient()
	client.listResult = []OverlayInstance{}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationList,
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	if resp.Result["count"] != 0 {
		t.Errorf("expected count=0, got %v", resp.Result["count"])
	}
}

func TestHandlerCreateResultFields(t *testing.T) {
	client := newDefaultFakeClient()
	now := time.Now().UnixMilli()
	client.createResult = OverlayInstance{
		ID:        "ovl_result_test",
		Kind:      OverlayKindCard,
		Visible:   true,
		Focusable: false,
		Touchable: true,
		X:         100,
		Y:         200,
		Width:     300,
		Height:    400,
		Gravity:   GravityCenter,
		CreatedAt: now,
		UpdatedAt: now,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationCreate,
		Payload: map[string]any{
			"kind": OverlayKindCard,
			"content": map[string]any{
				"title": "Test Card",
				"body":  "Test body",
			},
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	overlay, ok := resp.Result["overlay"].(OverlayInstance)
	if !ok {
		t.Fatalf("expected overlay in result, got %v", resp.Result)
	}
	if overlay.ID != "ovl_result_test" {
		t.Errorf("expected overlay ID ovl_result_test, got %s", overlay.ID)
	}
	if overlay.Kind != OverlayKindCard {
		t.Errorf("expected overlay kind=card, got %s", overlay.Kind)
	}
	if overlay.X != 100 {
		t.Errorf("expected overlay X=100, got %d", overlay.X)
	}
}

func TestHandlerUpdateResultFields(t *testing.T) {
	client := newDefaultFakeClient()
	client.updateResult = OverlayInstance{
		ID:      "ovl_update_test",
		Kind:    OverlayKindText,
		Visible: true,
		X:       50,
		Y:       60,
	}
	handler := NewOverlayHandler(client)

	resp := handler.Execute(context.Background(), androidsystem.NotificationRequest{
		RequestID: "test-1",
		Operation: OperationUpdate,
		Payload: map[string]any{
			"overlayId": "ovl_update_test",
			"content": map[string]any{
				"text": "Updated text",
			},
		},
	})

	if resp.Status != "success" {
		t.Errorf("expected success, got %s: %v", resp.Status, resp.Error)
	}
	overlay, ok := resp.Result["overlay"].(OverlayInstance)
	if !ok {
		t.Fatalf("expected overlay in result, got %v", resp.Result)
	}
	if overlay.ID != "ovl_update_test" {
		t.Errorf("expected overlay ID ovl_update_test, got %s", overlay.ID)
	}
}
