package interaction

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/androidnative/uitree"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockNodeResolver struct {
	resolveFunc func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error)
}

func (m *mockNodeResolver) ResolveNode(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, snapshotID, nodeID)
	}
	return uitree.ResolvedUINode{
		SnapshotID: snapshotID,
		Generation: 1,
		Node: uitree.UINode{
			NodeID:    nodeID,
			Clickable: true,
			Bounds:    uitree.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
		},
	}, nil
}

type mockSnapshotResolver struct {
	latestFunc   func(ctx context.Context) (uitree.UITreeSnapshot, error)
	getSnapFunc  func(ctx context.Context, snapshotID string) (uitree.UITreeSnapshot, error)
}

func (m *mockSnapshotResolver) Latest(ctx context.Context) (uitree.UITreeSnapshot, error) {
	if m.latestFunc != nil {
		return m.latestFunc(ctx)
	}
	return uitree.UITreeSnapshot{Generation: 1}, nil
}

func (m *mockSnapshotResolver) GetSnapshot(ctx context.Context, snapshotID string) (uitree.UITreeSnapshot, error) {
	if m.getSnapFunc != nil {
		return m.getSnapFunc(ctx, snapshotID)
	}
	return uitree.UITreeSnapshot{SnapshotID: snapshotID, Generation: 2}, nil
}

type mockAccessibilityExecutor struct {
	performFunc func(ctx context.Context, node uitree.ResolvedUINode, action string, args map[string]any) error
	supportFunc func(node uitree.ResolvedUINode, action string) bool
}

func (m *mockAccessibilityExecutor) PerformNodeAction(ctx context.Context, node uitree.ResolvedUINode, action string, args map[string]any) error {
	if m.performFunc != nil {
		return m.performFunc(ctx, node, action, args)
	}
	return nil
}

func (m *mockAccessibilityExecutor) SupportsAction(node uitree.ResolvedUINode, action string) bool {
	if m.supportFunc != nil {
		return m.supportFunc(node, action)
	}
	return true
}

type mockCoordinateExecutor struct {
	tapFunc       func(ctx context.Context, displayID, x, y int) error
	longPressFunc func(ctx context.Context, displayID, x, y int, duration time.Duration) error
	swipeFunc     func(ctx context.Context, request SwipeRequest) error
}

func (m *mockCoordinateExecutor) Tap(ctx context.Context, displayID, x, y int) error {
	if m.tapFunc != nil {
		return m.tapFunc(ctx, displayID, x, y)
	}
	return nil
}

func (m *mockCoordinateExecutor) LongPress(ctx context.Context, displayID, x, y int, duration time.Duration) error {
	if m.longPressFunc != nil {
		return m.longPressFunc(ctx, displayID, x, y, duration)
	}
	return nil
}

func (m *mockCoordinateExecutor) Swipe(ctx context.Context, request SwipeRequest) error {
	if m.swipeFunc != nil {
		return m.swipeFunc(ctx, request)
	}
	return nil
}

type mockVisualLocator struct {
	locateFunc func(ctx context.Context, request VisualLocateRequest) ([]VisualCandidate, error)
}

func (m *mockVisualLocator) Locate(ctx context.Context, request VisualLocateRequest) ([]VisualCandidate, error) {
	if m.locateFunc != nil {
		return m.locateFunc(ctx, request)
	}
	return []VisualCandidate{
		{
			Source:      StrategyVisualOCR,
			Text:        "Test",
			Bounds:      uitree.Rect{Left: 10, Top: 10, Right: 110, Bottom: 40},
			CenterX:     60,
			CenterY:     25,
			Confidence:  0.95,
		},
	}, nil
}

type mockRootExecutor struct {
	tapFunc  func(ctx context.Context, x, y int) error
	swipeFunc func(ctx context.Context, startX, startY, endX, endY, durationMS int) error
	inputFunc func(ctx context.Context, text string) error
}

func (m *mockRootExecutor) Tap(ctx context.Context, x, y int) error {
	if m.tapFunc != nil {
		return m.tapFunc(ctx, x, y)
	}
	return nil
}

func (m *mockRootExecutor) Swipe(ctx context.Context, startX, startY, endX, endY, durationMS int) error {
	if m.swipeFunc != nil {
		return m.swipeFunc(ctx, startX, startY, endX, endY, durationMS)
	}
	return nil
}

func (m *mockRootExecutor) InputText(ctx context.Context, text string) error {
	if m.inputFunc != nil {
		return m.inputFunc(ctx, text)
	}
	return nil
}

type mockADBExecutor struct {
	tapFunc  func(ctx context.Context, x, y int) error
	swipeFunc func(ctx context.Context, startX, startY, endX, endY, durationMS int) error
	inputFunc func(ctx context.Context, text string) error
}

func (m *mockADBExecutor) Tap(ctx context.Context, x, y int) error {
	if m.tapFunc != nil {
		return m.tapFunc(ctx, x, y)
	}
	return nil
}

func (m *mockADBExecutor) Swipe(ctx context.Context, startX, startY, endX, endY, durationMS int) error {
	if m.swipeFunc != nil {
		return m.swipeFunc(ctx, startX, startY, endX, endY, durationMS)
	}
	return nil
}

func (m *mockADBExecutor) InputText(ctx context.Context, text string) error {
	if m.inputFunc != nil {
		return m.inputFunc(ctx, text)
	}
	return nil
}

type mockVerifier struct {
	verifyFunc func(ctx context.Context, before InteractionContext, result InteractionResult) (VerificationResult, error)
}

func (m *mockVerifier) Verify(ctx context.Context, before InteractionContext, result InteractionResult) (VerificationResult, error) {
	if m.verifyFunc != nil {
		return m.verifyFunc(ctx, before, result)
	}
	return VerificationResult{Verified: true, Method: "mock"}, nil
}

func newTestService() *Service {
	return NewService(
		&mockNodeResolver{},
		&mockSnapshotResolver{},
		&mockAccessibilityExecutor{},
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		&mockRootExecutor{},
		&mockADBExecutor{},
		&mockVerifier{},
		DefaultPolicy(),
	)
}

func TestHandler_UnknownOperation(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "interaction.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestHandler_Status_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != INTERACTION_UNAVAILABLE {
		t.Fatalf("expected INTERACTION_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Status_Success(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationStatus,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["available"] != true {
		t.Fatalf("expected available true, got %v", resp.Result["available"])
	}
}

func TestHandler_Click_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClick,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != INTERACTION_UNAVAILABLE {
		t.Fatalf("expected INTERACTION_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestHandler_Click_NodeTarget(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClick,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "node_1",
			},
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["success"] != true {
		t.Fatalf("expected success true, got %v", resp.Result["success"])
	}
	strategy, _ := resp.Result["strategy"].(string)
	if strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", strategy)
	}
}

func TestHandler_Click_CoordinateTarget(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClick,
		Payload: map[string]any{
			"target": map[string]any{
				"x": 100,
				"y": 200,
			},
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	if resp.Result["success"] != true {
		t.Fatalf("expected success true, got %v", resp.Result["success"])
	}
}

func TestHandler_Click_NodeNotFound(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found"}
		},
	}
	service := NewService(
		nodeResolver,
		&mockSnapshotResolver{},
		&mockAccessibilityExecutor{},
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		&mockRootExecutor{},
		&mockADBExecutor{},
		&mockVerifier{},
		DefaultPolicy(),
	)
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClick,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "nonexistent",
			},
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != INTERACTION_NODE_NOT_FOUND {
		t.Fatalf("expected INTERACTION_NODE_NOT_FOUND, got %+v", resp.Error)
	}
}

func TestHandler_LongClick_NodeTarget(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationLongClick,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "node_1",
			},
			"durationMs": 800,
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestHandler_InputText_NodeTarget(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:   nodeID,
					Editable: true,
					Bounds:   uitree.Rect{Left: 0, Top: 0, Right: 200, Bottom: 50},
				},
			}, nil
		},
	}
	service := NewService(
		nodeResolver,
		&mockSnapshotResolver{},
		&mockAccessibilityExecutor{},
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		&mockRootExecutor{},
		&mockADBExecutor{},
		&mockVerifier{},
		DefaultPolicy(),
	)
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationInputText,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "node_1",
			},
			"text": "hello world",
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestHandler_InputText_PasswordFieldDenied(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:    nodeID,
					Editable:  true,
					Password:  true,
					Bounds:    uitree.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
				},
			}, nil
		},
	}
	service := NewService(
		nodeResolver,
		&mockSnapshotResolver{},
		&mockAccessibilityExecutor{},
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		&mockRootExecutor{},
		&mockADBExecutor{},
		&mockVerifier{},
		DefaultPolicy(),
	)
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationInputText,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "password_node",
			},
			"text": "secret",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.DomainCode != INTERACTION_SENSITIVE_INPUT_DENIED {
		t.Fatalf("expected INTERACTION_SENSITIVE_INPUT_DENIED, got %+v", resp.Error)
	}
}

func TestHandler_ClearText_NodeTarget(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:   nodeID,
					Editable: true,
					Bounds:   uitree.Rect{Left: 0, Top: 0, Right: 200, Bottom: 50},
				},
			}, nil
		},
	}
	service := NewService(
		nodeResolver,
		&mockSnapshotResolver{},
		&mockAccessibilityExecutor{},
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		&mockRootExecutor{},
		&mockADBExecutor{},
		&mockVerifier{},
		DefaultPolicy(),
	)
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClearText,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "node_1",
			},
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
}

func TestHandler_Scroll_NodeTarget(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationScroll,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "scrollable_node",
			},
			"direction": "down",
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
}

func TestHandler_Swipe_Coordinates(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationSwipe,
		Payload: map[string]any{
			"startX": 100,
			"startY": 200,
			"endX":   100,
			"endY":   400,
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestHandler_VisualLocate_NilService(t *testing.T) {
	handler := NewHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationVisualLocate,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_VisualLocate_Success(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationVisualLocate,
		Payload: map[string]any{
			"text": "Search",
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
	candidates, ok := resp.Result["count"].(int)
	if !ok || candidates < 1 {
		t.Fatalf("expected at least 1 candidate, got %v", resp.Result["count"])
	}
}

func TestHandler_VisualClick_Success(t *testing.T) {
	service := newTestService()
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationVisualClick,
		Payload: map[string]any{
			"text": "Login",
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestHandler_RootFallback(t *testing.T) {
	accessibility := &mockAccessibilityExecutor{
		supportFunc: func(node uitree.ResolvedUINode, action string) bool {
			return false
		},
		performFunc: func(ctx context.Context, node uitree.ResolvedUINode, action string, args map[string]any) error {
			return &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "not supported"}
		},
	}
	policy := DefaultPolicy()
	policy.AllowRootFallback = true
	policy.AllowADBFallback = false

	root := &mockRootExecutor{
		tapFunc: func(ctx context.Context, x, y int) error {
			return nil
		},
	}

	service := NewService(
		&mockNodeResolver{},
		&mockSnapshotResolver{},
		accessibility,
		nil,
		&mockVisualLocator{},
		root,
		nil,
		&mockVerifier{},
		policy,
	)
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClick,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "node_1",
			},
			"allowCoordinateFallback": true,
			"allowRootFallback":       true,
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
	strategy, _ := resp.Result["strategy"].(string)
	if strategy != StrategyRoot {
		t.Fatalf("expected root strategy fallback, got %s", strategy)
	}
}

func TestHandler_ADBFallback(t *testing.T) {
	accessibility := &mockAccessibilityExecutor{
		supportFunc: func(node uitree.ResolvedUINode, action string) bool {
			return false
		},
	}
	policy := DefaultPolicy()
	policy.AllowCoordinateFallback = false
	policy.AllowRootFallback = false
	policy.AllowADBFallback = true

	adb := &mockADBExecutor{
		tapFunc: func(ctx context.Context, x, y int) error {
			return nil
		},
	}

	service := NewService(
		&mockNodeResolver{},
		&mockSnapshotResolver{},
		accessibility,
		nil,
		&mockVisualLocator{},
		nil,
		adb,
		&mockVerifier{},
		policy,
	)
	handler := NewHandler(service)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       OperationClick,
		Payload: map[string]any{
			"target": map[string]any{
				"snapshotId": "snap_1",
				"nodeId":     "node_1",
			},
			"allowCoordinateFallback": true,
			"allowAdbFallback":        true,
		},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s: %+v", resp.Status, resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
	strategy, _ := resp.Result["strategy"].(string)
	if strategy != StrategyADB {
		t.Fatalf("expected adb strategy fallback, got %s", strategy)
	}
}

var _ AccessibilityExecutor = (*mockAccessibilityExecutor)(nil)
var _ CoordinateExecutor = (*mockCoordinateExecutor)(nil)
var _ VisualLocator = (*mockVisualLocator)(nil)
var _ RootInteractionExecutor = (*mockRootExecutor)(nil)
var _ ADBInteractionExecutor = (*mockADBExecutor)(nil)
var _ Verifier = (*mockVerifier)(nil)
var _ uitree.NodeResolver = (*mockNodeResolver)(nil)
var _ uitree.SnapshotResolver = (*mockSnapshotResolver)(nil)

type mockAndroidNativeBridge struct {
	executeFunc func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error)
	healthFunc  func(ctx context.Context) androidnative.NativeBridgeHealth
}

func (m *mockAndroidNativeBridge) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockAndroidNativeBridge) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return androidnative.NativeBridgeHealthReady
}

var _ androidnative.NativeBridge = (*mockAndroidNativeBridge)(nil)
