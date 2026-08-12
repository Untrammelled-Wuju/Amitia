package interaction

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

func TestService_Status_Available(t *testing.T) {
	service := newTestService()

	state := service.Status(context.Background())

	if !state.Available {
		t.Fatal("expected available to be true")
	}
	if !state.AccessibilityAction {
		t.Fatal("expected accessibility action to be true")
	}
	if state.State != "available" {
		t.Fatalf("expected state available, got %s", state.State)
	}
}

func TestService_Status_NoExecutors(t *testing.T) {
	policy := DefaultPolicy()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, policy)

	state := service.Status(context.Background())

	if state.Available {
		t.Fatal("expected available to be false when no executors")
	}
}

func TestService_Click_NodeTarget_AccessibilitySuccess(t *testing.T) {
	service := newTestService()

	req := ClickRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "node_1",
		},
	}

	result, err := service.Click(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", result.Strategy)
	}
	if result.Operation != OperationClick {
		t.Fatalf("expected click operation, got %s", result.Operation)
	}
}

func TestService_Click_NodeTarget_CoordinateFallback(t *testing.T) {
	accessibility := &mockAccessibilityExecutor{
		supportFunc: func(node uitree.ResolvedUINode, action string) bool {
			return false
		},
	}
	service := NewService(
		&mockNodeResolver{},
		&mockSnapshotResolver{},
		accessibility,
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := ClickRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "node_1",
		},
		AllowCoordinateFallback: true,
	}

	result, err := service.Click(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Strategy != StrategyNodeBounds {
		t.Fatalf("expected node_bounds strategy fallback, got %s", result.Strategy)
	}
}

func TestService_Click_NodeStale(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{}, &uitree.Error{Code: uitree.UI_NODE_STALE, Message: "stale"}
		},
	}
	service := NewService(
		nodeResolver,
		&mockSnapshotResolver{},
		&mockAccessibilityExecutor{},
		&mockCoordinateExecutor{},
		&mockVisualLocator{},
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := ClickRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "node_1",
		},
	}

	_, err := service.Click(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for stale node")
	}
	interErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if interErr.Code != INTERACTION_NODE_STALE {
		t.Fatalf("expected INTERACTION_NODE_STALE, got %s", interErr.Code)
	}
}

func TestService_Click_CoordinateTarget(t *testing.T) {
	service := newTestService()
	x, y := 150, 250

	req := ClickRequest{
		Target: InteractionTarget{
			X: &x,
			Y: &y,
		},
	}

	result, err := service.Click(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Strategy != StrategyCoordinate {
		t.Fatalf("expected coordinate strategy, got %s", result.Strategy)
	}
}

func TestService_CoordinateTarget_RootFallback(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowRootFallback = true

	service := NewService(
		&mockNodeResolver{},
		&mockSnapshotResolver{},
		nil,
		nil,
		&mockVisualLocator{},
		&mockRootExecutor{},
		nil,
		&mockVerifier{},
		policy,
	)

	x, y := 150, 250
	req := ClickRequest{
		Target: InteractionTarget{
			X: &x,
			Y: &y,
		},
		AllowRootFallback: true,
	}

	result, err := service.Click(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Strategy != StrategyRoot {
		t.Fatalf("expected root strategy fallback, got %s", result.Strategy)
	}
}

func TestService_LongClick_NodeTarget(t *testing.T) {
	service := newTestService()

	req := LongClickRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "node_1",
		},
		DurationMS: 800,
	}

	result, err := service.LongClick(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Operation != OperationLongClick {
		t.Fatalf("expected long_click operation, got %s", result.Operation)
	}
}

func TestService_LongClick_DurationClamping(t *testing.T) {
	service := newTestService()

	req := LongClickRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "node_1",
		},
		DurationMS: 50000,
	}

	result, err := service.LongClick(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success even with clamped duration")
	}
}

func TestService_InputText_EditableNode(t *testing.T) {
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
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := InputTextRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "input_node",
		},
		Text: "test input",
	}

	result, err := service.InputText(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Operation != OperationInputText {
		t.Fatalf("expected input_text operation, got %s", result.Operation)
	}
}

func TestService_InputText_PasswordDenied(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:   nodeID,
					Editable: true,
					Password: true,
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
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := InputTextRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "password_node",
		},
		Text: "secret",
	}

	_, err := service.InputText(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for password field input")
	}
	interErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if interErr.Code != INTERACTION_SENSITIVE_INPUT_DENIED {
		t.Fatalf("expected INTERACTION_SENSITIVE_INPUT_DENIED, got %s", interErr.Code)
	}
}

func TestService_InputText_NotEditable(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:   nodeID,
					Editable: false,
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
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := InputTextRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "label_node",
		},
		Text: "test",
	}

	_, err := service.InputText(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-editable node")
	}
}

func TestService_ClearText_EditableNode(t *testing.T) {
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
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := ClearTextRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "input_node",
		},
	}

	result, err := service.ClearText(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Operation != OperationClearText {
		t.Fatalf("expected clear_text operation, got %s", result.Operation)
	}
}

func TestService_Scroll_ForwardDirection(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:     nodeID,
					Scrollable: true,
					Bounds:     uitree.Rect{Left: 0, Top: 0, Right: 200, Bottom: 600},
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
		nil, nil, &mockVerifier{},
		DefaultPolicy(),
	)

	req := ScrollRequest{
		Target: InteractionTarget{
			SnapshotID: "snap_1",
			NodeID:     "scrollable_node",
		},
		Direction: DirectionForward,
	}

	result, err := service.Scroll(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestService_Swipe_Coordinate(t *testing.T) {
	service := newTestService()

	req := SwipeRequest{
		StartX:     100,
		StartY:     200,
		EndX:       100,
		EndY:       400,
		DurationMS: 300,
	}

	result, err := service.Swipe(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Strategy != StrategyCoordinate {
		t.Fatalf("expected coordinate strategy, got %s", result.Strategy)
	}
	if result.Operation != OperationSwipe {
		t.Fatalf("expected swipe operation, got %s", result.Operation)
	}
}

func TestService_Swipe_RootFallback(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowRootFallback = true

	service := NewService(
		nil, nil, nil, nil, nil,
		&mockRootExecutor{},
		nil, nil,
		policy,
	)

	req := SwipeRequest{
		StartX: 100,
		StartY: 200,
		EndX:   100,
		EndY:   400,
	}

	result, err := service.Swipe(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Strategy != StrategyRoot {
		t.Fatalf("expected root strategy, got %s", result.Strategy)
	}
}

func TestService_VisualLocate_Success(t *testing.T) {
	service := newTestService()

	req := VisualLocateRequest{
		Text: "Search Button",
	}

	candidates, err := service.VisualLocate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
}

func TestService_VisualLocate_NoLocator(t *testing.T) {
	service := NewService(
		nil, nil, nil, nil, nil, nil, nil, nil,
		DefaultPolicy(),
	)

	req := VisualLocateRequest{
		Text: "Search",
	}

	_, err := service.VisualLocate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing visual locator")
	}
}

func TestService_VisualClick_Success(t *testing.T) {
	service := newTestService()

	req := VisualClickRequest{
		Description: "a blue login button",
	}

	result, err := service.VisualClick(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Operation != OperationVisualClick {
		t.Fatalf("expected visual_click operation, got %s", result.Operation)
	}
}

func TestService_VisualClick_NoLocator(t *testing.T) {
	service := NewService(
		nil, nil, nil, nil, nil, nil, nil, nil,
		DefaultPolicy(),
	)

	req := VisualClickRequest{
		Description: "login button",
	}

	_, err := service.VisualClick(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing visual locator")
	}
}

func TestService_VisualClick_NotCoordinate(t *testing.T) {
	service := NewService(
		nil, nil,
		nil,
		nil,
		&mockVisualLocator{},
		nil, nil, nil,
		DefaultPolicy(),
	)

	req := VisualClickRequest{
		Description: "login button",
	}

	_, err := service.VisualClick(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing coordinate executor")
	}
}

func TestInteractionTarget_EffectiveTargetType(t *testing.T) {
	x, y := 100, 200

	tests := []struct {
		name     string
		target   InteractionTarget
		expected string
	}{
		{
			name:     "node target",
			target:   InteractionTarget{SnapshotID: "s1", NodeID: "n1"},
			expected: TargetNode,
		},
		{
			name:     "coordinate target",
			target:   InteractionTarget{X: &x, Y: &y},
			expected: TargetCoordinate,
		},
		{
			name:     "visual target",
			target:   InteractionTarget{Description: "login button"},
			expected: TargetVisual,
		},
		{
			name:     "empty target defaults to visual",
			target:   InteractionTarget{},
			expected: TargetVisual,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.target.EffectiveTargetType()
			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestInteractionTarget_HasNode(t *testing.T) {
	tests := []struct {
		name     string
		target   InteractionTarget
		expected bool
	}{
		{"valid node", InteractionTarget{SnapshotID: "s1", NodeID: "n1"}, true},
		{"missing snapshot", InteractionTarget{NodeID: "n1"}, false},
		{"missing node", InteractionTarget{SnapshotID: "s1"}, false},
		{"empty", InteractionTarget{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.target.HasNode()
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestInteractionTarget_HasCoordinate(t *testing.T) {
	x, y := 10, 20
	tests := []struct {
		name     string
		target   InteractionTarget
		expected bool
	}{
		{"valid coords", InteractionTarget{X: &x, Y: &y}, true},
		{"missing x", InteractionTarget{Y: &y}, false},
		{"missing y", InteractionTarget{X: &x}, false},
		{"empty", InteractionTarget{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.target.HasCoordinate()
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
