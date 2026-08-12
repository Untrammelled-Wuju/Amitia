package interaction

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type mockCoordinateExecutorLongPress struct {
	tapFunc       func(ctx context.Context, displayID, x, y int) error
	longPressFunc func(ctx context.Context, displayID, x, y int, duration time.Duration) error
	swipeFunc     func(ctx context.Context, request SwipeRequest) error
}

func (m *mockCoordinateExecutorLongPress) Tap(ctx context.Context, displayID, x, y int) error {
	if m.tapFunc != nil {
		return m.tapFunc(ctx, displayID, x, y)
	}
	return nil
}

func (m *mockCoordinateExecutorLongPress) LongPress(ctx context.Context, displayID, x, y int, duration time.Duration) error {
	if m.longPressFunc != nil {
		return m.longPressFunc(ctx, displayID, x, y, duration)
	}
	return nil
}

func (m *mockCoordinateExecutorLongPress) Swipe(ctx context.Context, request SwipeRequest) error {
	if m.swipeFunc != nil {
		return m.swipeFunc(ctx, request)
	}
	return nil
}

type mockBridgeForExecutor struct {
	executeFunc func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error)
	healthFunc  func(ctx context.Context) androidnative.NativeBridgeHealth
}

func (m *mockBridgeForExecutor) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockBridgeForExecutor) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return androidnative.NativeBridgeHealthReady
}

func TestStrategyResult_CanFallback(t *testing.T) {
	tests := []struct {
		name     string
		outcome  ExecutionOutcome
		expected bool
	}{
		{"success cannot fallback", OutcomeSuccess, false},
		{"unsupported can fallback", OutcomeUnsupported, true},
		{"failed can fallback", OutcomeDefinitelyFailed, true},
		{"unknown cannot fallback", OutcomeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := StrategyResult{Outcome: tt.outcome}
			if sr.CanFallback() != tt.expected {
				t.Fatalf("expected CanFallback %v, got %v", tt.expected, sr.CanFallback())
			}
		})
	}
}

func TestStrategyResult_IsUnknown(t *testing.T) {
	tests := []struct {
		name     string
		outcome  ExecutionOutcome
		expected bool
	}{
		{"success not unknown", OutcomeSuccess, false},
		{"unsupported not unknown", OutcomeUnsupported, false},
		{"failed not unknown", OutcomeDefinitelyFailed, false},
		{"unknown is unknown", OutcomeUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := StrategyResult{Outcome: tt.outcome}
			if sr.IsUnknown() != tt.expected {
				t.Fatalf("expected IsUnknown %v, got %v", tt.expected, sr.IsUnknown())
			}
		})
	}
}

func TestNewSuccessResult(t *testing.T) {
	result := InteractionResult{Success: true}
	sr := NewSuccessResult(result)
	if sr.Outcome != OutcomeSuccess {
		t.Fatalf("expected OutcomeSuccess, got %v", sr.Outcome)
	}
	if !sr.Result.Success {
		t.Fatal("expected result.Success to be true")
	}
}

func TestNewUnsupportedResult(t *testing.T) {
	sr := NewUnsupportedResult()
	if sr.Outcome != OutcomeUnsupported {
		t.Fatalf("expected OutcomeUnsupported, got %v", sr.Outcome)
	}
}

func TestNewFailureResult(t *testing.T) {
	err := &Error{Code: INTERACTION_ACTION_FAILED, Message: "failed"}
	sr := NewFailureResult(err)
	if sr.Outcome != OutcomeDefinitelyFailed {
		t.Fatalf("expected OutcomeDefinitelyFailed, got %v", sr.Outcome)
	}
	if sr.Error != err {
		t.Fatal("expected error to be set")
	}
}

func TestNewUnknownResult(t *testing.T) {
	sr := NewUnknownResult()
	if sr.Outcome != OutcomeUnknown {
		t.Fatalf("expected OutcomeUnknown, got %v", sr.Outcome)
	}
}

func TestBridgeAccessibilityExecutor_PerformNodeAction_Success(t *testing.T) {
	bridge := &mockBridgeForExecutor{}
	executor := NewBridgeAccessibilityExecutor(bridge)

	node := uitree.ResolvedUINode{
		SnapshotID: "snap_1",
		Generation: 1,
		NativeRef:  "ref_1",
		Node: uitree.UINode{
			NodeID:    "node_1",
			Clickable: true,
		},
	}

	err := executor.PerformNodeAction(context.Background(), node, NodeActionClick, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBridgeAccessibilityExecutor_PerformNodeAction_NilBridge(t *testing.T) {
	executor := NewBridgeAccessibilityExecutor(nil)

	node := uitree.ResolvedUINode{
		NativeRef: "ref_1",
	}

	err := executor.PerformNodeAction(context.Background(), node, NodeActionClick, nil)
	if err == nil {
		t.Fatal("expected error for nil bridge")
	}
	interErr, ok := err.(*Error)
	if !ok || interErr.Code != INTERACTION_NATIVE_HOST_UNAVAILABLE {
		t.Fatalf("expected INTERACTION_NATIVE_HOST_UNAVAILABLE, got %v", err)
	}
}

func TestBridgeAccessibilityExecutor_PerformNodeAction_EmptyNativeRef(t *testing.T) {
	executor := NewBridgeAccessibilityExecutor(&mockBridgeForExecutor{})

	node := uitree.ResolvedUINode{
		NativeRef: "",
	}

	err := executor.PerformNodeAction(context.Background(), node, NodeActionClick, nil)
	if err == nil {
		t.Fatal("expected error for empty native ref")
	}
	interErr, ok := err.(*Error)
	if !ok || interErr.Code != INTERACTION_NODE_STALE {
		t.Fatalf("expected INTERACTION_NODE_STALE, got %v", err)
	}
}

func TestBridgeAccessibilityExecutor_PerformNodeAction_BridgeError(t *testing.T) {
	bridge := &mockBridgeForExecutor{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "error",
				Error: &androidnative.NativeBridgeError{
					Code:    "ACTION_FAILED",
					Message: "action failed on device",
				},
			}, nil
		},
	}
	executor := NewBridgeAccessibilityExecutor(bridge)

	node := uitree.ResolvedUINode{
		NativeRef: "ref_1",
	}

	err := executor.PerformNodeAction(context.Background(), node, NodeActionClick, nil)
	if err == nil {
		t.Fatal("expected error for bridge error response")
	}
}

func TestBridgeAccessibilityExecutor_PerformNodeAction_UnsupportedMessage(t *testing.T) {
	bridge := &mockBridgeForExecutor{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "error",
				Error: &androidnative.NativeBridgeError{
					Code:    "UNSUPPORTED",
					Message: "action is unsupported on this node",
				},
			}, nil
		},
	}
	executor := NewBridgeAccessibilityExecutor(bridge)

	node := uitree.ResolvedUINode{
		NativeRef: "ref_1",
	}

	err := executor.PerformNodeAction(context.Background(), node, NodeActionClick, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	interErr, ok := err.(*Error)
	if !ok || interErr.Code != INTERACTION_ACTION_UNSUPPORTED {
		t.Fatalf("expected INTERACTION_ACTION_UNSUPPORTED, got %v", err)
	}
}

func TestBridgeAccessibilityExecutor_PerformNodeAction_StaleMessage(t *testing.T) {
	bridge := &mockBridgeForExecutor{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "error",
				Error: &androidnative.NativeBridgeError{
					Code:    "STALE",
					Message: "node reference is stale and expired",
				},
			}, nil
		},
	}
	executor := NewBridgeAccessibilityExecutor(bridge)

	node := uitree.ResolvedUINode{
		NativeRef: "ref_1",
	}

	err := executor.PerformNodeAction(context.Background(), node, NodeActionClick, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	interErr, ok := err.(*Error)
	if !ok || interErr.Code != INTERACTION_NODE_STALE {
		t.Fatalf("expected INTERACTION_NODE_STALE, got %v", err)
	}
}

func TestBridgeAccessibilityExecutor_SupportsAction(t *testing.T) {
	executor := NewBridgeAccessibilityExecutor(&mockBridgeForExecutor{})

	tests := []struct {
		name     string
		node     uitree.ResolvedUINode
		action   string
		expected bool
	}{
		{
			name:     "clickable node supports click",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Clickable: true}, NativeRef: "ref"},
			action:   NodeActionClick,
			expected: true,
		},
		{
			name:     "non-clickable node without click action",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Clickable: false}, NativeRef: "ref"},
			action:   NodeActionClick,
			expected: false,
		},
		{
			name:     "node with click in actions list",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Actions: []string{"click"}}, NativeRef: "ref"},
			action:   NodeActionClick,
			expected: true,
		},
		{
			name:     "editable node supports set_text",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Editable: true}, NativeRef: "ref"},
			action:   NodeActionSetText,
			expected: true,
		},
		{
			name:     "editable node supports clear_text",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Editable: true}, NativeRef: "ref"},
			action:   NodeActionClearText,
			expected: true,
		},
		{
			name:     "scrollable node supports scroll_forward",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Scrollable: true}, NativeRef: "ref"},
			action:   NodeActionScrollForward,
			expected: true,
		},
		{
			name:     "focusable node supports focus",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{Focusable: true}, NativeRef: "ref"},
			action:   NodeActionFocus,
			expected: true,
		},
		{
			name:     "empty native ref supports nothing",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{}, NativeRef: ""},
			action:   NodeActionClick,
			expected: false,
		},
		{
			name:     "select is always supported",
			node:     uitree.ResolvedUINode{Node: uitree.UINode{}, NativeRef: "ref"},
			action:   NodeActionSelect,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.SupportsAction(tt.node, tt.action)
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestContainsAction(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		target   string
		expected bool
	}{
		{"empty actions", nil, "click", false},
		{"exact match", []string{"click", "long_click"}, "click", true},
		{"case insensitive", []string{"Click", "Long_Click"}, "click", true},
		{"no match", []string{"focus", "select"}, "click", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAction(tt.actions, tt.target)
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestIsUnsupportedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"unsupported error", &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "nope"}, true},
		{"other error", &Error{Code: INTERACTION_ACTION_FAILED, Message: "failed"}, false},
		{"non-interaction error", &dummyError{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnsupportedError(tt.err)
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

type dummyError struct{}

func (e *dummyError) Error() string { return "dummy" }

func TestIsUnsupportedError_NonInteraction(t *testing.T) {
	regularErr := &Error{Code: INTERACTION_ACTION_FAILED, Message: "test"}
	if isUnsupportedError(regularErr) {
		t.Fatal("should return false for non-unsupported error")
	}
	if isUnsupportedError(nil) {
		t.Fatal("should return false for nil error")
	}
}

func TestCoordinateExecutor_Swipe_WithMock(t *testing.T) {
	called := false
	coord := &mockCoordinateExecutorLongPress{
		swipeFunc: func(ctx context.Context, request SwipeRequest) error {
			called = true
			if request.StartX != 100 || request.StartY != 200 {
				t.Fatalf("unexpected coordinates: (%d, %d)", request.StartX, request.StartY)
			}
			return nil
		},
	}

	err := coord.Swipe(context.Background(), SwipeRequest{
		StartX: 100,
		StartY: 200,
		EndX:   300,
		EndY:   400,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected Swipe to be called")
	}
}
