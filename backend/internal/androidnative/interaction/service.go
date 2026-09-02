package interaction

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type Service struct {
	nodeResolver     uitree.NodeResolver
	snapshotResolver uitree.SnapshotResolver

	accessibility AccessibilityExecutor
	coordinate    CoordinateExecutor

	visual VisualLocator

	root    RootInteractionExecutor
	adb     ADBInteractionExecutor
	shizuku ShizukuInteractionExecutor

	verifier Verifier

	policy Policy

	routeMu     sync.Mutex
	routeStats  map[string]providerRouteStats
	routeHealth map[string]providerHealthCacheEntry
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
		routeStats:       make(map[string]providerRouteStats),
		routeHealth:      make(map[string]providerHealthCacheEntry),
	}
}

func (s *Service) Status(ctx context.Context) CapabilityState {
	state := CapabilityState{
		State:       "unavailable",
		HealthState: ProviderStateUnavailable,
		Providers:   make(map[string]ProviderCapabilityHealth),
	}

	accessibilityHealth := probeProviderHealth(ctx, "accessibility", s.accessibility)
	coordinateHealth := probeProviderHealth(ctx, "accessibility_gesture", s.coordinate)
	shizukuHealth := probeProviderHealth(ctx, "shizuku", s.shizuku)
	rootHealth := probeProviderHealth(ctx, "root", s.root)
	adbHealth := probeProviderHealth(ctx, "adb", s.adb)
	state.Providers["accessibility"] = accessibilityHealth
	state.Providers["accessibilityGesture"] = coordinateHealth
	state.Providers["shizuku"] = shizukuHealth
	state.Providers["root"] = rootHealth
	state.Providers["adb"] = adbHealth

	state.AccessibilityAction = providerUsable(accessibilityHealth)
	state.AccessibilityGesture = providerUsable(coordinateHealth)
	state.CoordinateTap = providerUsable(coordinateHealth)
	state.Shizuku = s.policy.AllowShizukuFallback && providerUsable(shizukuHealth)
	state.RootFallback = s.policy.AllowRootFallback && providerUsable(rootHealth)
	state.ADBFallback = s.policy.AllowADBFallback && providerUsable(adbHealth)

	state.TextInput = state.AccessibilityAction || state.Shizuku || state.RootFallback || state.ADBFallback
	state.Scroll = state.AccessibilityAction || state.AccessibilityGesture || state.Shizuku || state.RootFallback || state.ADBFallback

	if s.visual != nil {
		if probe, ok := s.visual.(VisualCapabilityProbe); ok {
			visualState := probe.ProviderState(ctx)
			state.VisualLocate = visualState.ScreenshotAvailable && (visualState.OCRAvailable || visualState.ImageUnderstandAvailable)
			state.OCRAvailable = visualState.OCRAvailable
			state.ImageUnderstandAvailable = visualState.ImageUnderstandAvailable
			visualHealth := newProviderHealth("visual", ProviderStateUnavailable, visualState.Reason, "", true)
			if state.VisualLocate {
				visualHealth = newProviderHealth("visual", ProviderStateReady, "", "", true)
			} else if visualState.ScreenshotAvailable {
				visualHealth = newProviderHealth("visual", ProviderStateDegraded, visualState.Reason, "", true)
			}
			state.Providers["visual"] = visualHealth
			if visualState.OCRAvailable {
				state.Providers["ocr"] = newProviderHealth("ocr", ProviderStateReady, "", "", true)
			} else {
				state.Providers["ocr"] = newProviderHealth("ocr", ProviderStateUnavailable, "OCR provider unavailable", "", true)
			}
			if visualState.ImageUnderstandAvailable {
				state.Providers["vision"] = newProviderHealth("vision", ProviderStateReady, "", "", true)
			} else {
				state.Providers["vision"] = newProviderHealth("vision", ProviderStateUnavailable, "image understanding provider unavailable", "", true)
			}
			if !state.VisualLocate && state.Reason == "" {
				state.Reason = visualState.Reason
			}
		} else {
			state.Providers["visual"] = newProviderHealth("visual", ProviderStateSupported, "visual locator does not expose provider health", "", true)
			state.VisualLocate = true
		}
	} else {
		state.Providers["visual"] = newProviderHealth("visual", ProviderStateUnavailable, "visual locator not configured", "", true)
	}

	state.Available = state.AccessibilityAction || state.CoordinateTap || state.Shizuku || state.RootFallback || state.ADBFallback || state.VisualLocate

	readyCount := 0
	degradedCount := 0
	permissionCount := 0
	failedCount := 0
	for _, health := range state.Providers {
		switch health.State {
		case ProviderStateReady:
			readyCount++
		case ProviderStateSupported, ProviderStateDegraded, ProviderStateStarting:
			degradedCount++
		case ProviderStatePermissionRequired:
			permissionCount++
		case ProviderStateFailed:
			failedCount++
		}
	}

	switch {
	case state.Available && failedCount == 0:
		state.State = "available"
		if readyCount > 0 {
			state.HealthState = ProviderStateReady
		} else {
			state.HealthState = ProviderStateDegraded
		}
	case state.Available:
		state.State = "degraded"
		state.HealthState = ProviderStateDegraded
	case permissionCount > 0:
		state.State = "unavailable"
		state.HealthState = ProviderStatePermissionRequired
	case failedCount > 0:
		state.State = "unavailable"
		state.HealthState = ProviderStateFailed
	case degradedCount > 0:
		state.State = "degraded"
		state.HealthState = ProviderStateDegraded
	default:
		state.State = "unavailable"
		state.HealthState = ProviderStateUnavailable
	}

	if state.Reason == "" && !state.Available {
		for _, name := range []string{"accessibility", "accessibilityGesture", "shizuku", "adb", "root", "visual"} {
			if health, ok := state.Providers[name]; ok && health.Reason != "" {
				state.Reason = health.Reason
				break
			}
		}
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

	bounds := node.Node.Bounds
	displayID := s.nodeDisplayID(ctx, node)
	centerX, centerY := bounds.CenterX(), bounds.CenterY()
	candidates := make([]providerRouteCandidate, 0, 5)
	if s.accessibility != nil && s.accessibility.SupportsAction(node, NodeActionClick) {
		candidates = append(candidates, providerRouteCandidate{
			name: "accessibility", strategy: StrategyAccessibilityAction, provider: s.accessibility, baseScore: 120,
			execute: func() error { return s.accessibility.PerformNodeAction(ctx, node, NodeActionClick, nil) },
		})
	}
	if req.AllowCoordinateFallback && bounds.Width() > 0 && bounds.Height() > 0 {
		if s.coordinate != nil {
			candidates = append(candidates, providerRouteCandidate{
				name: "accessibility_gesture", strategy: StrategyNodeBounds, provider: s.coordinate, baseScore: 100,
				execute: func() error { return s.coordinate.Tap(ctx, displayID, centerX, centerY) },
			})
		}
		if displayID == 0 && req.AllowShizukuFallback && s.policy.AllowShizukuFallback && s.shizuku != nil {
			candidates = append(candidates, providerRouteCandidate{name: "shizuku", strategy: StrategyShizuku, provider: s.shizuku, baseScore: 95, execute: func() error { return s.shizuku.Tap(ctx, centerX, centerY) }})
		}
		if displayID == 0 && req.AllowRootFallback && s.policy.AllowRootFallback && s.root != nil {
			candidates = append(candidates, providerRouteCandidate{name: "root", strategy: StrategyRoot, provider: s.root, baseScore: 90, execute: func() error { return s.root.Tap(ctx, centerX, centerY) }})
		}
		if displayID == 0 && req.AllowADBFallback && s.policy.AllowADBFallback && s.adb != nil {
			candidates = append(candidates, providerRouteCandidate{name: "adb", strategy: StrategyADB, provider: s.adb, baseScore: 80, execute: func() error { return s.adb.Tap(ctx, centerX, centerY) }})
		}
	}

	strategy, err := s.executeRankedProvider(ctx, candidates)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{
		Success: true, Operation: OperationClick, Strategy: strategy,
		SnapshotID: target.SnapshotID, NodeID: target.NodeID, DisplayID: displayID,
		DurationMS: time.Since(startTime).Milliseconds(),
	}
	if strategy != StrategyAccessibilityAction {
		result.X, result.Y = &centerX, &centerY
	}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
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
	centerX, centerY := bounds.CenterX(), bounds.CenterY()
	displayID := s.nodeDisplayID(ctx, node)
	strategy, err := s.executeTapRouted(ctx, displayID, centerX, centerY,
		req.AllowCoordinateFallback, req.AllowShizukuFallback, req.AllowRootFallback, req.AllowADBFallback)
	if err != nil {
		return InteractionResult{}, err
	}
	if strategy == StrategyCoordinate {
		strategy = StrategyNodeBounds
	}
	result := InteractionResult{Success: true, Operation: OperationClick, Strategy: strategy, SnapshotID: node.SnapshotID, NodeID: node.Node.NodeID, DisplayID: displayID, X: &centerX, Y: &centerY, DurationMS: time.Since(startTime).Milliseconds()}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
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
	x, y := *target.X, *target.Y
	strategy, err := s.executeTapRouted(ctx, target.DisplayID, x, y, true, req.AllowShizukuFallback, req.AllowRootFallback, req.AllowADBFallback)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{Success: true, Operation: OperationClick, Strategy: strategy, DisplayID: target.DisplayID, X: &x, Y: &y, DurationMS: time.Since(startTime).Milliseconds()}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
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
		DisplayID:       0,
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

	if validator, ok := s.visual.(VisualCandidateValidator); ok {
		if err := validator.ValidateCandidate(ctx, best); err != nil {
			return InteractionResult{}, err
		}
	}

	x := best.CenterX
	y := best.CenterY
	strategy, err := s.executeTapRouted(ctx, best.DisplayID, x, y, true, req.AllowShizukuFallback, req.AllowRootFallback, req.AllowADBFallback)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{
		Success:                  true,
		Operation:                OperationClick,
		Strategy:                 best.Source + ":" + strategy,
		DisplayID:                best.DisplayID,
		X:                        &x,
		Y:                        &y,
		DurationMS:               time.Since(startTime).Milliseconds(),
		BaselineScreenStateToken: best.ScreenStateToken,
	}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
}

func (s *Service) LongClick(ctx context.Context, req LongClickRequest) (InteractionResult, error) {
	startTime := time.Now()
	target := req.Target
	if target.EffectiveTargetType() != TargetNode {
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
	bounds := node.Node.Bounds
	displayID := s.nodeDisplayID(ctx, node)
	centerX, centerY := bounds.CenterX(), bounds.CenterY()
	candidates := make([]providerRouteCandidate, 0, 5)
	if s.accessibility != nil && s.accessibility.SupportsAction(node, NodeActionLongClick) {
		candidates = append(candidates, providerRouteCandidate{name: "accessibility", strategy: StrategyAccessibilityAction, provider: s.accessibility, baseScore: 120, execute: func() error {
			return s.accessibility.PerformNodeAction(ctx, node, NodeActionLongClick, map[string]any{"durationMs": durationMS})
		}})
	}
	if req.AllowCoordinateFallback && bounds.Width() > 0 && bounds.Height() > 0 && s.coordinate != nil {
		candidates = append(candidates, providerRouteCandidate{name: "accessibility_gesture", strategy: StrategyNodeBounds, provider: s.coordinate, baseScore: 100, execute: func() error {
			return s.coordinate.LongPress(ctx, displayID, centerX, centerY, time.Duration(durationMS)*time.Millisecond)
		}})
	}
	if displayID == 0 && req.AllowShizukuFallback && s.policy.AllowShizukuFallback && s.shizuku != nil {
		candidates = append(candidates, providerRouteCandidate{name: "shizuku", strategy: StrategyShizuku, provider: s.shizuku, baseScore: 95, execute: func() error { return s.shizuku.LongPress(ctx, centerX, centerY, durationMS) }})
	}
	if displayID == 0 && req.AllowRootFallback && s.policy.AllowRootFallback && s.root != nil {
		candidates = append(candidates, providerRouteCandidate{name: "root", strategy: StrategyRoot, provider: s.root, baseScore: 90, execute: func() error { return s.root.Swipe(ctx, centerX, centerY, centerX, centerY, durationMS) }})
	}
	if displayID == 0 && req.AllowADBFallback && s.policy.AllowADBFallback && s.adb != nil {
		candidates = append(candidates, providerRouteCandidate{name: "adb", strategy: StrategyADB, provider: s.adb, baseScore: 80, execute: func() error { return s.adb.Swipe(ctx, centerX, centerY, centerX, centerY, durationMS) }})
	}
	strategy, err := s.executeRankedProvider(ctx, candidates)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{Success: true, Operation: OperationLongClick, Strategy: strategy, SnapshotID: target.SnapshotID, NodeID: target.NodeID, DisplayID: displayID, DurationMS: time.Since(startTime).Milliseconds()}
	if strategy != StrategyAccessibilityAction {
		result.X, result.Y = &centerX, &centerY
	}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
}

func (s *Service) InputText(ctx context.Context, req InputTextRequest) (InteractionResult, error) {
	startTime := time.Now()
	target := req.Target
	if target.EffectiveTargetType() != TargetNode {
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

	candidates := make([]providerRouteCandidate, 0, 4)
	if s.accessibility != nil {
		candidates = append(candidates, providerRouteCandidate{name: "accessibility", strategy: StrategyAccessibilityAction, provider: s.accessibility, baseScore: 120, execute: func() error {
			return s.accessibility.PerformNodeAction(ctx, node, NodeActionSetText, map[string]any{"text": req.Text})
		}})
	}
	if s.policy.AllowShizukuFallback && s.shizuku != nil {
		candidates = append(candidates, providerRouteCandidate{name: "shizuku", strategy: StrategyShizuku, provider: s.shizuku, baseScore: 100, execute: func() error { return s.shizuku.InputText(ctx, req.Text) }})
	}
	if s.policy.AllowRootFallback && s.root != nil {
		candidates = append(candidates, providerRouteCandidate{name: "root", strategy: StrategyRoot, provider: s.root, baseScore: 90, execute: func() error { return s.root.InputText(ctx, req.Text) }})
	}
	if req.AllowADBFallback && s.policy.AllowADBFallback && s.adb != nil {
		candidates = append(candidates, providerRouteCandidate{name: "adb", strategy: StrategyADB, provider: s.adb, baseScore: 80, execute: func() error { return s.adb.InputText(ctx, req.Text) }})
	}
	strategy, err := s.executeRankedProvider(ctx, candidates)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{Success: true, Operation: OperationInputText, Strategy: strategy, SnapshotID: target.SnapshotID, NodeID: target.NodeID, DurationMS: time.Since(startTime).Milliseconds()}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
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
				s.verifyResult(ctx, &result)
			}
			return result, nil
		}
	}

	return InteractionResult{}, &Error{Code: INTERACTION_TEXT_INPUT_UNSUPPORTED, Message: "clear text not supported"}
}

func (s *Service) Scroll(ctx context.Context, req ScrollRequest) (InteractionResult, error) {
	startTime := time.Now()
	target := req.Target
	if target.EffectiveTargetType() != TargetNode {
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
	action := NodeActionScrollForward
	switch req.Direction {
	case DirectionBackward, DirectionUp, DirectionLeft:
		action = NodeActionScrollBackward
	}
	bounds := node.Node.Bounds
	displayID := s.nodeDisplayID(ctx, node)
	startX, startY, endX, endY := bounds.CenterX(), bounds.CenterY(), bounds.CenterX(), bounds.CenterY()
	if bounds.Width() > 0 && bounds.Height() > 0 {
		switch req.Direction {
		case DirectionForward, DirectionDown:
			startY, endY = bounds.Bottom-10, bounds.Top+10
		case DirectionBackward, DirectionUp:
			startY, endY = bounds.Top+10, bounds.Bottom-10
		case DirectionRight:
			startX, endX = bounds.Right-10, bounds.Left+10
		case DirectionLeft:
			startX, endX = bounds.Left+10, bounds.Right-10
		}
	}
	candidates := make([]providerRouteCandidate, 0, 5)
	if s.accessibility != nil && s.accessibility.SupportsAction(node, action) {
		candidates = append(candidates, providerRouteCandidate{name: "accessibility", strategy: StrategyAccessibilityAction, provider: s.accessibility, baseScore: 120, execute: func() error { return s.accessibility.PerformNodeAction(ctx, node, action, nil) }})
	}
	if bounds.Width() > 0 && bounds.Height() > 0 && s.coordinate != nil {
		swipe := SwipeRequest{DisplayID: displayID, StartX: startX, StartY: startY, EndX: endX, EndY: endY, DurationMS: DefaultSwipeDurationMS}
		candidates = append(candidates, providerRouteCandidate{name: "accessibility_gesture", strategy: StrategyNodeBounds, provider: s.coordinate, baseScore: 100, execute: func() error { return s.coordinate.Swipe(ctx, swipe) }})
		if displayID == 0 && s.policy.AllowShizukuFallback && s.shizuku != nil {
			candidates = append(candidates, providerRouteCandidate{name: "shizuku", strategy: StrategyShizuku, provider: s.shizuku, baseScore: 95, execute: func() error { return s.shizuku.Swipe(ctx, startX, startY, endX, endY, DefaultSwipeDurationMS) }})
		}
		if displayID == 0 && s.policy.AllowRootFallback && s.root != nil {
			candidates = append(candidates, providerRouteCandidate{name: "root", strategy: StrategyRoot, provider: s.root, baseScore: 90, execute: func() error { return s.root.Swipe(ctx, startX, startY, endX, endY, DefaultSwipeDurationMS) }})
		}
		if displayID == 0 && s.policy.AllowADBFallback && s.adb != nil {
			candidates = append(candidates, providerRouteCandidate{name: "adb", strategy: StrategyADB, provider: s.adb, baseScore: 80, execute: func() error { return s.adb.Swipe(ctx, startX, startY, endX, endY, DefaultSwipeDurationMS) }})
		}
	}
	strategy, err := s.executeRankedProvider(ctx, candidates)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{Success: true, Operation: OperationScroll, Strategy: strategy, SnapshotID: target.SnapshotID, NodeID: target.NodeID, DisplayID: displayID, DurationMS: time.Since(startTime).Milliseconds()}
	if strategy != StrategyAccessibilityAction {
		result.X, result.Y = &startX, &startY
	}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
}

func (s *Service) Swipe(ctx context.Context, req SwipeRequest) (InteractionResult, error) {
	startTime := time.Now()
	strategy, err := s.executeSwipeRouted(ctx, req, true, true, true, true)
	if err != nil {
		return InteractionResult{}, err
	}
	return InteractionResult{Success: true, Operation: OperationSwipe, Strategy: strategy, DisplayID: req.DisplayID, X: &req.StartX, Y: &req.StartY, DurationMS: time.Since(startTime).Milliseconds()}, nil
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
		DisplayID:       req.DisplayID,
		Description:     req.Description,
		Text:            req.Text,
		Role:            req.Role,
		ExpectedPackage: req.ExpectedPackage,
		OCRFirst:        req.OCRFirst,
		TextMatchMode:   req.TextMatchMode,
	}

	candidates, err := s.visual.Locate(ctx, locateReq)
	if err != nil {
		return InteractionResult{}, err
	}

	if len(candidates) == 0 {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_NOT_FOUND, Message: "no visual target found"}
	}

	best := candidates[0]

	if len(candidates) >= 2 {
		if best.Confidence-candidates[1].Confidence < 0.1 {
			return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_AMBIGUOUS, Message: "visual target is ambiguous"}
		}
	}

	if best.Confidence < s.policy.MinVisionConfidence {
		return InteractionResult{}, &Error{Code: INTERACTION_VISUAL_TARGET_AMBIGUOUS, Message: "visual target confidence too low"}
	}

	if validator, ok := s.visual.(VisualCandidateValidator); ok {
		if err := validator.ValidateCandidate(ctx, best); err != nil {
			return InteractionResult{}, err
		}
	}

	x := best.CenterX
	y := best.CenterY
	strategy, err := s.executeTapRouted(ctx, best.DisplayID, x, y, true, true, true, true)
	if err != nil {
		return InteractionResult{}, err
	}
	result := InteractionResult{
		Success:                  true,
		Operation:                OperationVisualClick,
		Strategy:                 best.Source + ":" + strategy,
		DisplayID:                best.DisplayID,
		X:                        &x,
		Y:                        &y,
		DurationMS:               time.Since(startTime).Milliseconds(),
		BaselineScreenStateToken: best.ScreenStateToken,
	}
	if req.Verify {
		s.verifyResult(ctx, &result)
	}
	return result, nil
}

func (s *Service) nodeDisplayID(ctx context.Context, node ResolvedUINode) int {
	if s.snapshotResolver == nil || node.SnapshotID == "" || node.Node.WindowID == "" {
		return 0
	}
	snapshot, err := s.snapshotResolver.GetSnapshot(ctx, node.SnapshotID)
	if err != nil {
		return 0
	}
	for _, window := range snapshot.Windows {
		if window.WindowID == node.Node.WindowID {
			return window.DisplayID
		}
	}
	return 0
}

func (s *Service) verifyResult(ctx context.Context, result *InteractionResult) {
	if s.verifier == nil {
		return
	}

	before := InteractionContext{
		ExpectedPackage:  "",
		ExpectedWindowID: result.SnapshotID,
		Timestamp:        time.Now().Add(-time.Duration(result.DurationMS) * time.Millisecond),
	}

	verifyResult, err := s.verifier.Verify(ctx, before, *result)
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
