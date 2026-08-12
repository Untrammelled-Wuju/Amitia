package interaction

import (
	"context"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type Planner struct {
	nodeResolver uitree.NodeResolver
	visualLocator VisualLocator
	policy       Policy
}

func NewPlanner(
	nodeResolver uitree.NodeResolver,
	visualLocator VisualLocator,
	policy Policy,
) *Planner {
	return &Planner{
		nodeResolver:  nodeResolver,
		visualLocator: visualLocator,
		policy:        policy,
	}
}

func (p *Planner) Plan(
	ctx context.Context,
	target InteractionTarget,
	operation string,
	allowCoordinateFallback bool,
	allowVisualFallback bool,
) (InteractionPlan, error) {
	targetType := target.EffectiveTargetType()

	switch targetType {
	case TargetNode:
		return p.planNodeAction(ctx, target, operation)
	case TargetCoordinate:
		return p.planCoordinateAction(target, operation), nil
	case TargetVisual:
		return p.planVisualAction(target, operation), nil
	default:
		return InteractionPlan{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "unknown target type"}
	}
}

func (p *Planner) planNodeAction(
	ctx context.Context,
	target InteractionTarget,
	operation string,
) (InteractionPlan, error) {
	if !target.HasNode() {
		return InteractionPlan{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "node target requires snapshotId and nodeId"}
	}

	if p.nodeResolver == nil {
		return InteractionPlan{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node resolver not available"}
	}

	node, err := p.nodeResolver.ResolveNode(ctx, target.SnapshotID, target.NodeID)
	if err != nil {
		if treeErr, ok := err.(*uitree.Error); ok && treeErr.Code == uitree.UI_NODE_STALE {
			return InteractionPlan{}, &Error{Code: INTERACTION_NODE_STALE, Message: "node is stale"}
		}
		return InteractionPlan{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found: " + err.Error()}
	}

	strategy := p.selectNodeStrategy(node, operation)

	if strategy != "" {
		return InteractionPlan{
			Operation:  operation,
			Strategy:   strategy,
			SnapshotID: target.SnapshotID,
			NodeID:     target.NodeID,
			DisplayID:  0,
		}, nil
	}

	return InteractionPlan{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "no suitable strategy for node action"}
}

func (p *Planner) selectNodeStrategy(
	node ResolvedUINode,
	operation string,
) string {
	switch operation {
	case OperationClick:
		if node.Node.Clickable || containsAction(node.Node.Actions, "click") {
			return StrategyAccessibilityAction
		}
		if node.Node.Bounds.Width() > 0 && node.Node.Bounds.Height() > 0 {
			return StrategyNodeBounds
		}
	case OperationLongClick:
		if node.Node.LongClickable || containsAction(node.Node.Actions, "long_click") {
			return StrategyAccessibilityAction
		}
		if node.Node.Bounds.Width() > 0 && node.Node.Bounds.Height() > 0 {
			return StrategyNodeBounds
		}
	case OperationInputText:
		if node.Node.Editable {
			return StrategyAccessibilityAction
		}
	case OperationClearText:
		if node.Node.Editable {
			return StrategyAccessibilityAction
		}
	case OperationScroll:
		if node.Node.Scrollable || containsAction(node.Node.Actions, "scroll_forward") || containsAction(node.Node.Actions, "scroll_backward") {
			return StrategyAccessibilityAction
		}
		if node.Node.Bounds.Width() > 0 && node.Node.Bounds.Height() > 0 {
			return StrategyNodeBounds
		}
	}

	return ""
}

func (p *Planner) planCoordinateAction(
	target InteractionTarget,
	operation string,
) InteractionPlan {
	displayID := 0
	x := 0
	y := 0
	if target.X != nil {
		x = *target.X
	}
	if target.Y != nil {
		y = *target.Y
	}

	return InteractionPlan{
		Operation: operation,
		Strategy:  StrategyCoordinate,
		DisplayID: displayID,
		X:         x,
		Y:         y,
	}
}

func (p *Planner) planVisualAction(
	target InteractionTarget,
	operation string,
) InteractionPlan {
	return InteractionPlan{
		Operation: operation,
		Strategy:  StrategyVisualUnderstand,
	}
}

