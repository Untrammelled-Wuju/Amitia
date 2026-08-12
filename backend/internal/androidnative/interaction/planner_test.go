package interaction

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

func TestPlanner_Plan_NodeTarget_Click(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:    nodeID,
					Clickable: true,
					Bounds:    uitree.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
				},
			}, nil
		},
	}

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())

	target := InteractionTarget{
		SnapshotID: "snap_1",
		NodeID:     "node_1",
	}

	plan, err := planner.Plan(context.Background(), target, OperationClick, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", plan.Strategy)
	}
	if plan.Operation != OperationClick {
		t.Fatalf("expected click operation, got %s", plan.Operation)
	}
}

func TestPlanner_Plan_NodeTarget_Click_FallbackToBounds(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:    nodeID,
					Clickable: false,
					Bounds:    uitree.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
				},
			}, nil
		},
	}

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())

	target := InteractionTarget{
		SnapshotID: "snap_1",
		NodeID:     "node_1",
	}

	plan, err := planner.Plan(context.Background(), target, OperationClick, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyNodeBounds {
		t.Fatalf("expected node_bounds strategy, got %s", plan.Strategy)
	}
}

func TestPlanner_Plan_NodeTarget_Scroll(t *testing.T) {
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

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())

	target := InteractionTarget{
		SnapshotID: "snap_1",
		NodeID:     "scrollable_node",
	}

	plan, err := planner.Plan(context.Background(), target, OperationScroll, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", plan.Strategy)
	}
}

func TestPlanner_Plan_NodeTarget_InputText(t *testing.T) {
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

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())

	target := InteractionTarget{
		SnapshotID: "snap_1",
		NodeID:     "input_node",
	}

	plan, err := planner.Plan(context.Background(), target, OperationInputText, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", plan.Strategy)
	}
}

func TestPlanner_Plan_NodeTarget_NodeNotFound(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "not found"}
		},
	}

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{SnapshotID: "snap_1", NodeID: "bad_node"}

	_, err := planner.Plan(context.Background(), target, OperationClick, true, true)
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestPlanner_Plan_NodeTarget_StaleNode(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{}, &uitree.Error{Code: uitree.UI_NODE_STALE, Message: "stale"}
		},
	}

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{SnapshotID: "snap_1", NodeID: "stale_node"}

	_, err := planner.Plan(context.Background(), target, OperationClick, true, true)
	if err == nil {
		t.Fatal("expected error for stale node")
	}
	interErr, ok := err.(*Error)
	if !ok || interErr.Code != INTERACTION_NODE_STALE {
		t.Fatalf("expected INTERACTION_NODE_STALE, got %v", err)
	}
}

func TestPlanner_Plan_NodeTarget_MissingIDs(t *testing.T) {
	planner := NewPlanner(&mockNodeResolver{}, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{NodeID: "node_without_snapshot"}

	_, err := planner.Plan(context.Background(), target, OperationClick, true, true)
	if err == nil {
		t.Fatal("expected error for missing snapshot ID")
	}
}

func TestPlanner_Plan_NodeTarget_NilResolver(t *testing.T) {
	planner := NewPlanner(nil, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{SnapshotID: "snap_1", NodeID: "node_1"}

	_, err := planner.Plan(context.Background(), target, OperationClick, true, true)
	if err == nil {
		t.Fatal("expected error for nil node resolver")
	}
}

func TestPlanner_Plan_CoordinateTarget(t *testing.T) {
	planner := NewPlanner(&mockNodeResolver{}, &mockVisualLocator{}, DefaultPolicy())
	x, y := 150, 250
	target := InteractionTarget{X: &x, Y: &y}

	plan, err := planner.Plan(context.Background(), target, OperationClick, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyCoordinate {
		t.Fatalf("expected coordinate strategy, got %s", plan.Strategy)
	}
	if plan.X != 150 || plan.Y != 250 {
		t.Fatalf("expected (150, 250), got (%d, %d)", plan.X, plan.Y)
	}
}

func TestPlanner_Plan_VisualTarget(t *testing.T) {
	planner := NewPlanner(&mockNodeResolver{}, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{Description: "login button"}

	plan, err := planner.Plan(context.Background(), target, OperationVisualClick, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyVisualUnderstand {
		t.Fatalf("expected visual_understand strategy, got %s", plan.Strategy)
	}
}

func TestPlanner_Plan_LongClick(t *testing.T) {
	nodeResolver := &mockNodeResolver{
		resolveFunc: func(ctx context.Context, snapshotID, nodeID string) (uitree.ResolvedUINode, error) {
			return uitree.ResolvedUINode{
				SnapshotID: snapshotID,
				Generation: 1,
				Node: uitree.UINode{
					NodeID:        nodeID,
					LongClickable: true,
					Bounds:        uitree.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
				},
			}, nil
		},
	}

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{SnapshotID: "snap_1", NodeID: "node_1"}

	plan, err := planner.Plan(context.Background(), target, OperationLongClick, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", plan.Strategy)
	}
}

func TestPlanner_Plan_ClearText(t *testing.T) {
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

	planner := NewPlanner(nodeResolver, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{SnapshotID: "snap_1", NodeID: "input_node"}

	plan, err := planner.Plan(context.Background(), target, OperationClearText, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Strategy != StrategyAccessibilityAction {
		t.Fatalf("expected accessibility_action strategy, got %s", plan.Strategy)
	}
}

func TestPlanner_Plan_UnsupportedOperation(t *testing.T) {
	planner := NewPlanner(&mockNodeResolver{}, &mockVisualLocator{}, DefaultPolicy())
	target := InteractionTarget{SnapshotID: "snap_1", NodeID: "node_1"}

	_, err := planner.Plan(context.Background(), target, "unknown.operation", true, true)
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}
}
