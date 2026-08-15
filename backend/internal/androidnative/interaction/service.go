package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type Service struct {
	nodeResolver  uitree.NodeResolver
	snapshotResolver uitree.SnapshotResolver

	accessibility AccessibilityExecutor
	coordinate    CoordinateExecutor

	visual VisualLocator

	root     RootInteractionExecutor
	adb      ADBInteractionExecutor
	shizuku  ShizukuInteractionExecutor

	verifier Verifier

	policy Policy
}

func NewService(
	nodeResolver uitree.NodeResolver,
	snapshotResolver uitree.SnapshotResolver,
	accessibility AccessibilityExecutor,
	coordinate CoordinateExecutor,
	visual VisualLocator,
	root RootInteractionExecutor,
	adb ADBInteractionExecutor,
	shizuku ShizukuInteractionExecutor,
	verifier Verifier,
	policy Policy,
) *Service {
	return &Service{
		nodeResolver:     nodeResolver,
		snapshotResolver: snapshotResolver,
		accessibility:    accessibility,
		coordinate:       coordinate,
		visual:           visual,
		root:             root,
		adb:              adb,
		shizuku:          shizuku,
		verifier:         verifier,
		policy:           policy,
	}
}

func (s *Service) Status(ctx context.Context) CapabilityState {
	state := CapabilityState{
		State: "unavailable",
	}

	if s.accessibility != nil {
		state.AccessibilityAction = true
		state.AccessibilityGesture = true
		state.Available = true
		state.State = "available"
	}

	if s.coordinate != nil {
		state.CoordinateTap = true
		state.Available = true
		if state.State != "available" {
			state.State = "degraded"
		}
	}

	state.TextInput = s.accessibility != nil
	state.Scroll = s.accessibility != nil

	if s.visual != nil {
		state.VisualLocate = true
		state.OCRAvailable = true
		state.ImageUnderstandAvailable = true
	}

	if s.root != nil && s.policy.AllowRootFallback {
		state.RootFallback = true
	}

	if s.adb != nil && s.policy.AllowADBFallback {
		state.ADBFallback = true
	}

	return state
}

func (s *Service) Click(ctx context.Context, req ClickRequest) (InteractionResult, error) {
	startTime := time.Now()

	target := req.Target
	targetType := target.EffectiveTargetType()

	switch targetType {
	case TargetNode:
		return s.clickNode(ctx, target, req, startTime)
	case TargetCoordinate:
		return s.clickCoordinate(ctx, target, req, startTime)
	case TargetVisual:
		if req.AllowVisualFallback && s.visual != nil {
			return s.clickVisual(ctx, target, req, startTime)
		}
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "visual fallback not allowed"}
	default:
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "unknown target type"}
	}
}

func (s *Service) clickNode(
	ctx context.Context,
	target InteractionTarget,
	req ClickRequest,
	startTime time.Time,
) (InteractionResult, error) {
	if s.nodeResolver == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node resolver not available"}
	}

	node, err := s.nodeResolver.ResolveNode(ctx, target.SnapshotID, target.NodeID)
	if err != nil {
		if treeErr, ok := err.(*uitree.Error); ok && treeErr.Code == uitree.UI_NODE_STALE {
			return InteractionResult{}, &Error{Code: INTERACTION_NODE_STALE, Message: "node is stale"}
		}
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found"}
	}

	if s.accessibility != nil && s.accessibility.SupportsAction(node, NodeActionClick) {
		err := s.accessibility.PerformNodeAction(ctx, node, NodeActionClick, nil)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyAccessibilityAction,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		if isUnsupportedError(err) {
			if req.AllowCoordinateFallback {
				return s.clickNodeBounds(ctx, node, req, startTime)
			}
		}
		return InteractionResult{}, err
	}

	if req.AllowCoordinateFallback {
		return s.clickNodeBounds(ctx, node, req, startTime)
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "click not supported on node"}
}

func (s *Service) clickNodeBounds(
	ctx context.Context,
	node ResolvedUINode,
	req ClickRequest,
	startTime time.Time,
) (InteractionResult, error) {
	bounds := node.Node.Bounds
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return InteractionResult{}, &Error{Code: INTERACTION_COORDINATE_INVALID, Message: "node has no valid bounds"}
	}

	centerX := bounds.CenterX()
	centerY := bounds.CenterY()

	if s.coordinate != nil {
		err := s.coordinate.Tap(ctx, 0, centerX, centerY)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyNodeBounds,
				SnapshotID: node.SnapshotID,
				NodeID:     node.Node.NodeID,
				X:          &centerX,
				Y:          &centerY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	if s.root != nil && req.AllowRootFallback && s.policy.AllowRootFallback {
		err := s.root.Tap(ctx, centerX, centerY)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyRoot,
				SnapshotID: node.SnapshotID,
				NodeID:     node.Node.NodeID,
				X:          &centerX,
				Y:          &centerY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	if s.adb != nil && req.AllowADBFallback && s.policy.AllowADBFallback {
		err := s.adb.Tap(ctx, centerX, centerY)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyADB,
				SnapshotID: node.SnapshotID,
				NodeID:     node.Node.NodeID,
				X:          &centerX,
				Y:          &centerY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "no coordinate executor available"}
}

func (s *Service) clickCoordinate(
	ctx context.Context,
	target InteractionTarget,
	req ClickRequest,
	startTime time.Time,
) (InteractionResult, error) {
	if !target.HasCoordinate() {
		return InteractionResult{}, &Error{Code: INTERACTION_COORDINATE_INVALID, Message: "invalid coordinate target"}
	}

	x := *target.X
	y := *target.Y

	if s.coordinate != nil {
		err := s.coordinate.Tap(ctx, 0, x, y)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyCoordinate,
				X:          &x,
				Y:          &y,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	if s.root != nil && req.AllowRootFallback && s.policy.AllowRootFallback {
		err := s.root.Tap(ctx, x, y)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyRoot,
				X:          &x,
				Y:          &y,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	if s.adb != nil && req.AllowADBFallback && s.policy.AllowADBFallback {
		err := s.adb.Tap(ctx, x, y)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   StrategyADB,
				X:          &x,
				Y:          &y,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "no coordinate executor available"}
}

func (s *Service) clickVisual(
	ctx context.Context,
	target InteractionTarget,
	req ClickRequest,
	startTime time.Time,
) (InteractionResult, error) {
	if s.visual == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "visual locator not available"}
	}

	locateReq := VisualLocateRequest{
		Description:     target.Description,
		Text:            target.Text,
		Role:            target.Role,
		ExpectedPackage: "",
	}

	candidates, err := s.visual.Locate(ctx, locateReq)
	if err != nil {
		return InteractionResult{}, err
	}

	if len(candidates) == 0 {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no visual target found"}
	}

	best := candidates[0]
	if best.Confidence < s.policy.MinVisionConfidence {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_AMBIGUOUS, Message: "visual target confidence too low"}
	}

	x := best.CenterX
	y := best.CenterY

	if s.coordinate != nil {
		err := s.coordinate.Tap(ctx, 0, x, y)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClick,
				Strategy:   best.Source,
				X:          &x,
				Y:          &y,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "no coordinate executor available for visual click"}
}

func (s *Service) LongClick(ctx context.Context, req LongClickRequest) (InteractionResult, error) {
	startTime := time.Now()

	target := req.Target
	targetType := target.EffectiveTargetType()

	if targetType != TargetNode {
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "long click currently only supports node target"}
	}

	if s.nodeResolver == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node resolver not available"}
	}

	node, err := s.nodeResolver.ResolveNode(ctx, target.SnapshotID, target.NodeID)
	if err != nil {
		if treeErr, ok := err.(*uitree.Error); ok && treeErr.Code == uitree.UI_NODE_STALE {
			return InteractionResult{}, &Error{Code: INTERACTION_NODE_STALE, Message: "node is stale"}
		}
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found"}
	}

	durationMS := req.DurationMS
	if durationMS <= 0 {
		durationMS = DefaultLongPressDurationMS
	}
	if durationMS < MinLongPressDurationMS {
		durationMS = MinLongPressDurationMS
	}
	if durationMS > MaxLongPressDurationMS {
		durationMS = MaxLongPressDurationMS
	}

	if s.accessibility != nil && s.accessibility.SupportsAction(node, NodeActionLongClick) {
		err := s.accessibility.PerformNodeAction(ctx, node, NodeActionLongClick, map[string]any{
			"durationMs": durationMS,
		})
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationLongClick,
				Strategy:   StrategyAccessibilityAction,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
	}

	if req.AllowCoordinateFallback && s.coordinate != nil {
		bounds := node.Node.Bounds
		centerX := bounds.CenterX()
		centerY := bounds.CenterY()
		duration := time.Duration(durationMS) * time.Millisecond

		err := s.coordinate.LongPress(ctx, 0, centerX, centerY, duration)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationLongClick,
				Strategy:   StrategyNodeBounds,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				X:          &centerX,
				Y:          &centerY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "long click not supported"}
}

func (s *Service) InputText(ctx context.Context, req InputTextRequest) (InteractionResult, error) {
	startTime := time.Now()

	target := req.Target
	targetType := target.EffectiveTargetType()

	if targetType != TargetNode {
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "input text currently only supports node target"}
	}

	if s.nodeResolver == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node resolver not available"}
	}

	node, err := s.nodeResolver.ResolveNode(ctx, target.SnapshotID, target.NodeID)
	if err != nil {
		if treeErr, ok := err.(*uitree.Error); ok && treeErr.Code == uitree.UI_NODE_STALE {
			return InteractionResult{}, &Error{Code: INTERACTION_NODE_STALE, Message: "node is stale"}
		}
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found"}
	}

	if node.Node.Password {
		return InteractionResult{}, &Error{Code: INTERACTION_SENSITIVE_INPUT_DENIED, Message: "password field input denied"}
	}

	if !node.Node.Editable {
		return InteractionResult{}, &Error{Code: INTERACTION_TEXT_INPUT_UNSUPPORTED, Message: "node is not editable"}
	}

	if len([]rune(req.Text)) > MaxInputTextRunes {
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "text too large"}
	}

	if s.accessibility != nil {
		err := s.accessibility.PerformNodeAction(ctx, node, NodeActionSetText, map[string]any{
			"text": req.Text,
		})
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationInputText,
				Strategy:   StrategyAccessibilityAction,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
	}

	if req.AllowADBFallback && s.adb != nil && s.policy.AllowADBFallback {
		err := s.adb.InputText(ctx, req.Text)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationInputText,
				Strategy:   StrategyADB,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_TEXT_INPUT_UNSUPPORTED, Message: "text input not supported"}
}

func (s *Service) ClearText(ctx context.Context, req ClearTextRequest) (InteractionResult, error) {
	startTime := time.Now()

	target := req.Target
	targetType := target.EffectiveTargetType()

	if targetType != TargetNode {
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "clear text currently only supports node target"}
	}

	if s.nodeResolver == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node resolver not available"}
	}

	node, err := s.nodeResolver.ResolveNode(ctx, target.SnapshotID, target.NodeID)
	if err != nil {
		if treeErr, ok := err.(*uitree.Error); ok && treeErr.Code == uitree.UI_NODE_STALE {
			return InteractionResult{}, &Error{Code: INTERACTION_NODE_STALE, Message: "node is stale"}
		}
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found"}
	}

	if !node.Node.Editable {
		return InteractionResult{}, &Error{Code: INTERACTION_TEXT_INPUT_UNSUPPORTED, Message: "node is not editable"}
	}

	if s.accessibility != nil {
		err := s.accessibility.PerformNodeAction(ctx, node, NodeActionClearText, nil)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationClearText,
				Strategy:   StrategyAccessibilityAction,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
	}

	return InteractionResult{}, &Error{Code: INTERACTION_TEXT_INPUT_UNSUPPORTED, Message: "clear text not supported"}
}

func (s *Service) Scroll(ctx context.Context, req ScrollRequest) (InteractionResult, error) {
	startTime := time.Now()

	target := req.Target
	targetType := target.EffectiveTargetType()

	if targetType != TargetNode {
		return InteractionResult{}, &Error{Code: INTERACTION_INVALID_REQUEST, Message: "scroll currently only supports node target"}
	}

	if s.nodeResolver == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node resolver not available"}
	}

	node, err := s.nodeResolver.ResolveNode(ctx, target.SnapshotID, target.NodeID)
	if err != nil {
		if treeErr, ok := err.(*uitree.Error); ok && treeErr.Code == uitree.UI_NODE_STALE {
			return InteractionResult{}, &Error{Code: INTERACTION_NODE_STALE, Message: "node is stale"}
		}
		return InteractionResult{}, &Error{Code: INTERACTION_NODE_NOT_FOUND, Message: "node not found"}
	}

	var action string
	switch req.Direction {
	case DirectionForward, DirectionDown, DirectionRight:
		action = NodeActionScrollForward
	case DirectionBackward, DirectionUp, DirectionLeft:
		action = NodeActionScrollBackward
	default:
		action = NodeActionScrollForward
	}

	if s.accessibility != nil && s.accessibility.SupportsAction(node, action) {
		err := s.accessibility.PerformNodeAction(ctx, node, action, nil)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationScroll,
				Strategy:   StrategyAccessibilityAction,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
	}

	bounds := node.Node.Bounds
	if bounds.Width() > 0 && bounds.Height() > 0 && s.coordinate != nil {
		startX := bounds.CenterX()
		startY := bounds.CenterY()
		endX := startX
		endY := startY

		switch req.Direction {
		case DirectionForward, DirectionDown:
			startY = bounds.Bottom - 10
			endY = bounds.Top + 10
		case DirectionBackward, DirectionUp:
			startY = bounds.Top + 10
			endY = bounds.Bottom - 10
		case DirectionRight:
			startX = bounds.Right - 10
			endX = bounds.Left + 10
		case DirectionLeft:
			startX = bounds.Left + 10
			endX = bounds.Right - 10
		}

		err := s.coordinate.Swipe(ctx, SwipeRequest{
			DisplayID:  0,
			StartX:     startX,
			StartY:     startY,
			EndX:       endX,
			EndY:       endY,
			DurationMS: DefaultSwipeDurationMS,
		})
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationScroll,
				Strategy:   StrategyNodeBounds,
				SnapshotID: target.SnapshotID,
				NodeID:     target.NodeID,
				X:          &startX,
				Y:          &startY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "scroll not supported"}
}

func (s *Service) Swipe(ctx context.Context, req SwipeRequest) (InteractionResult, error) {
	startTime := time.Now()

	if s.coordinate != nil {
		err := s.coordinate.Swipe(ctx, req)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationSwipe,
				Strategy:   StrategyCoordinate,
				DisplayID:  req.DisplayID,
				X:          &req.StartX,
				Y:          &req.StartY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	if s.root != nil && s.policy.AllowRootFallback {
		err := s.root.Swipe(ctx, req.StartX, req.StartY, req.EndX, req.EndY, req.DurationMS)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationSwipe,
				Strategy:   StrategyRoot,
				DisplayID:  req.DisplayID,
				X:          &req.StartX,
				Y:          &req.StartY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	if s.adb != nil && s.policy.AllowADBFallback {
		err := s.adb.Swipe(ctx, req.StartX, req.StartY, req.EndX, req.EndY, req.DurationMS)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationSwipe,
				Strategy:   StrategyADB,
				DisplayID:  req.DisplayID,
				X:          &req.StartX,
				Y:          &req.StartY,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "swipe not supported"}
}

func (s *Service) VisualLocate(ctx context.Context, req VisualLocateRequest) ([]VisualCandidate, error) {
	if s.visual == nil {
		return nil, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "visual locator not available"}
	}

	return s.visual.Locate(ctx, req)
}

func (s *Service) VisualClick(ctx context.Context, req VisualClickRequest) (InteractionResult, error) {
	startTime := time.Now()

	if s.visual == nil {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_UNAVAILABLE, Message: "visual locator not available"}
	}

	locateReq := VisualLocateRequest{
		Description:     req.Description,
		Text:            req.Text,
		Role:            req.Role,
		ExpectedPackage: req.ExpectedPackage,
		OCRFirst:        req.OCRFirst,
	}

	candidates, err := s.visual.Locate(ctx, locateReq)
	if err != nil {
		return InteractionResult{}, err
	}

	if len(candidates) == 0 {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no visual target found"}
	}

	best := candidates[0]

	if len(candidates) > 1 && len(candidates) >= 2 {
		if candidates[1].Confidence-best.Confidence < 0.1 {
			return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_AMBIGUOUS, Message: "visual target is ambiguous"}
		}
	}

	if best.Confidence < s.policy.MinVisionConfidence {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_AMBIGUOUS, Message: "visual target confidence too low"}
	}

	x := best.CenterX
	y := best.CenterY

	if s.coordinate != nil {
		err := s.coordinate.Tap(ctx, 0, x, y)
		if err == nil {
			result := InteractionResult{
				Success:    true,
				Operation:  OperationVisualClick,
				Strategy:   best.Source,
				X:          &x,
				Y:          &y,
				DurationMS: time.Since(startTime).Milliseconds(),
			}
			if req.Verify {
				s.verifyResult(ctx, result)
			}
			return result, nil
		}
		return InteractionResult{}, err
	}

	return InteractionResult{}, &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: "no coordinate executor available for visual click"}
}

func (s *Service) verifyResult(ctx context.Context, result InteractionResult) {
	if s.verifier == nil {
		return
	}

	before := InteractionContext{
		ExpectedPackage: "",
		ExpectedWindowID: result.SnapshotID,
		Timestamp:       time.Now().Add(-time.Duration(result.DurationMS) * time.Millisecond),
	}

	verifyResult, err := s.verifier.Verify(ctx, before, result)
	if err == nil {
		result.Verified = verifyResult.Verified
		result.Verification = verifyResult.Method
	}
}

func isUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	if interErr, ok := err.(*Error); ok {
		return interErr.Code == INTERACTION_ACTION_UNSUPPORTED
	}
	return false
}
