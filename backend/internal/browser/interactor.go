package browser

import (
	"context"
	"strings"
	"sync"
	"time"
)

type productionInteractor struct {
	tabs     TabResolver
	dom      DOMBackend
	input    InputBackend
	elements *elementStore
	policy   *InteractionPolicy
	tabMgr   *productionTabManager
	mu       sync.RWMutex
}

func NewProductionInteractor(tabs TabResolver, dom DOMBackend, input InputBackend, elements *elementStore, policy *InteractionPolicy, tabMgr *productionTabManager) BrowserInteractor {
	return &productionInteractor{
		tabs:     tabs,
		dom:      dom,
		input:    input,
		elements: elements,
		policy:   policy,
		tabMgr:   tabMgr,
	}
}

func (i *productionInteractor) Click(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef) (*BrowserInteractionResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if element.StableID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "element stableId is required",
		}
	}

	startTime := time.Now()
	i.mu.Lock()
	defer i.mu.Unlock()

	resolved, err := i.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	record, ok := i.elements.get(element.StableID)
	if !ok {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element ref not found",
		}
	}

	if record.sessionID != sessionID || record.tabID != tabID {
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to this session/tab",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		i.elements.remove(element.StableID)
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale runtime generation",
		}
	}

	if err := i.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	if err := i.dom.ScrollIntoView(ctx, resolved.TargetID, record.backendNodeID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to scroll element into view",
			Cause:   err,
		}
	}

	quads, domErr := i.dom.GetContentQuads(ctx, resolved.TargetID, record.backendNodeID)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotInteractable,
			Message: "element has no visible layout",
			Cause:   domErr,
		}
	}

	x, y := pickClickPoint(quads)
	if x < 0 || y < 0 {
		return nil, &BrowserError{
			Code:    ErrCodeElementOccluded,
			Message: "element is not visible or fully occluded",
		}
	}

	if err := i.input.DispatchMouseMove(ctx, resolved.TargetID, x, y); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to move mouse",
			Cause:   err,
		}
	}

	if err := i.input.DispatchMouseDown(ctx, resolved.TargetID, x, y, "left", 1); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to press mouse button",
			Cause:   err,
		}
	}

	if err := i.input.DispatchMouseUp(ctx, resolved.TargetID, x, y, "left", 1); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to release mouse button",
			Cause:   err,
		}
	}

	var docGen uint64
	if i.tabMgr != nil {
		if rec, ok := i.tabMgr.store.get(tabID); ok {
			docGen = rec.documentGeneration
		}
	}

	return &BrowserInteractionResult{
		Success:            true,
		Action:             "click",
		Strategy:           "cdp_input",
		Verified:           true,
		DocumentGeneration: docGen,
		DurationMS:         time.Since(startTime).Milliseconds(),
	}, nil
}

func (i *productionInteractor) Input(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef, text string) (*BrowserInteractionResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if element.StableID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "element stableId is required",
		}
	}

	if err := i.policy.ValidateInputText(text); err != nil {
		return nil, err
	}

	startTime := time.Now()
	i.mu.Lock()
	defer i.mu.Unlock()

	resolved, err := i.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	record, ok := i.elements.get(element.StableID)
	if !ok {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element ref not found",
		}
	}

	if record.sessionID != sessionID || record.tabID != tabID {
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to this session/tab",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		i.elements.remove(element.StableID)
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale runtime generation",
		}
	}

	if err := i.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	if err := i.dom.ScrollIntoView(ctx, resolved.TargetID, record.backendNodeID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to scroll element into view",
			Cause:   err,
		}
	}

	if err := i.dom.Focus(ctx, resolved.TargetID, record.backendNodeID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to focus element",
			Cause:   err,
		}
	}

	objectID, domErr := i.dom.ResolveNode(ctx, resolved.TargetID, record.backendNodeID)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to resolve node for input",
			Cause:   domErr,
		}
	}

	setValueHelper := (&InteractionHelpers{}).SetInputValue()
	setValueExpr := "function(){" + strings.Replace(setValueHelper, "function setValue(value)", "function()", 1) + "}"
	fullExpr := setValueExpr + ".call(arguments[0], arguments[1])"

	if err := i.dom.CallFunctionOn(ctx, resolved.TargetID, objectID, fullExpr, map[string]string{"key": "value", "value": text}); err != nil {
		if err := i.input.InsertText(ctx, resolved.TargetID, text); err != nil {
			return nil, &BrowserError{
				Code:    ErrCodeInteractionFailed,
				Message: "failed to input text",
				Cause:   err,
			}
		}
	}

	var docGen uint64
	if i.tabMgr != nil {
		if rec, ok := i.tabMgr.store.get(tabID); ok {
			docGen = rec.documentGeneration
		}
	}

	return &BrowserInteractionResult{
		Success:            true,
		Action:             "input",
		Strategy:           "cdp_insert_text",
		Verified:           true,
		DocumentGeneration: docGen,
		DurationMS:         time.Since(startTime).Milliseconds(),
	}, nil
}

func (i *productionInteractor) Select(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef, value string) (*BrowserInteractionResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if element.StableID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "element stableId is required",
		}
	}

	startTime := time.Now()
	i.mu.Lock()
	defer i.mu.Unlock()

	resolved, err := i.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	record, ok := i.elements.get(element.StableID)
	if !ok {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element ref not found",
		}
	}

	if record.sessionID != sessionID || record.tabID != tabID {
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to this session/tab",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		i.elements.remove(element.StableID)
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale runtime generation",
		}
	}

	if err := i.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	if err := i.dom.ScrollIntoView(ctx, resolved.TargetID, record.backendNodeID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to scroll element into view",
			Cause:   err,
		}
	}

	objectID, domErr := i.dom.ResolveNode(ctx, resolved.TargetID, record.backendNodeID)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to resolve node for select",
			Cause:   domErr,
		}
	}

	selectHelper := (&InteractionHelpers{}).SetSelectValue()
	selectExpr := "function(){" + strings.Replace(selectHelper, "function setSelectValue(value)", "function()", 1) + "}"
	fullExpr := selectExpr + ".call(arguments[0], arguments[1])"

	if err := i.dom.CallFunctionOn(ctx, resolved.TargetID, objectID, fullExpr, map[string]string{"key": "value", "value": value}); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to set select value",
			Cause:   err,
		}
	}

	var docGen uint64
	if i.tabMgr != nil {
		if rec, ok := i.tabMgr.store.get(tabID); ok {
			docGen = rec.documentGeneration
		}
	}

	return &BrowserInteractionResult{
		Success:            true,
		Action:             "select",
		Strategy:           "js_helper",
		Verified:           true,
		DocumentGeneration: docGen,
		DurationMS:         time.Since(startTime).Milliseconds(),
	}, nil
}

func (i *productionInteractor) Hover(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, element BrowserElementRef) (*BrowserInteractionResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if element.StableID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "element stableId is required",
		}
	}

	startTime := time.Now()
	i.mu.Lock()
	defer i.mu.Unlock()

	resolved, err := i.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	record, ok := i.elements.get(element.StableID)
	if !ok {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotFound,
			Message: "element ref not found",
		}
	}

	if record.sessionID != sessionID || record.tabID != tabID {
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element does not belong to this session/tab",
		}
	}

	if record.runtimeGeneration != resolved.RuntimeGeneration {
		i.elements.remove(element.StableID)
		return nil, &BrowserError{
			Code:    ErrCodeStaleElement,
			Message: "element belongs to stale runtime generation",
		}
	}

	if err := i.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	if err := i.dom.ScrollIntoView(ctx, resolved.TargetID, record.backendNodeID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to scroll element into view",
			Cause:   err,
		}
	}

	quads, domErr := i.dom.GetContentQuads(ctx, resolved.TargetID, record.backendNodeID)
	if domErr != nil {
		return nil, &BrowserError{
			Code:    ErrCodeElementNotInteractable,
			Message: "element has no visible layout",
			Cause:   domErr,
		}
	}

	x, y := pickClickPoint(quads)
	if x < 0 || y < 0 {
		return nil, &BrowserError{
			Code:    ErrCodeElementOccluded,
			Message: "element is not visible or fully occluded",
		}
	}

	if err := i.input.DispatchMouseMove(ctx, resolved.TargetID, x, y); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to move mouse for hover",
			Cause:   err,
		}
	}

	var docGen uint64
	if i.tabMgr != nil {
		if rec, ok := i.tabMgr.store.get(tabID); ok {
			docGen = rec.documentGeneration
		}
	}

	return &BrowserInteractionResult{
		Success:            true,
		Action:             "hover",
		Strategy:           "cdp_input",
		Verified:           true,
		DocumentGeneration: docGen,
		DurationMS:         time.Since(startTime).Milliseconds(),
	}, nil
}

func (i *productionInteractor) Scroll(ctx context.Context, sessionID BrowserSessionID, tabID BrowserTabID, direction string) (*BrowserInteractionResult, *BrowserError) {
	if err := ctx.Err(); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "context cancelled",
			Cause:   err,
		}
	}

	if !i.policy.IsScrollDirectionAllowed(direction) {
		return nil, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "invalid scroll direction: " + direction,
		}
	}

	startTime := time.Now()
	i.mu.Lock()
	defer i.mu.Unlock()

	resolved, err := i.tabs.ResolveTab(ctx, sessionID, tabID)
	if err != nil {
		return nil, err
	}

	if err := i.dom.EnableDOM(ctx, resolved.TargetID); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to enable DOM",
			Cause:   err,
		}
	}

	var deltaX, deltaY int64
	switch direction {
	case "up":
		deltaY = -300
	case "down":
		deltaY = 300
	case "left":
		deltaX = -300
	case "right":
		deltaX = 300
	}

	if err := i.input.DispatchMouseWheel(ctx, resolved.TargetID, deltaX, deltaY); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeInteractionFailed,
			Message: "failed to dispatch scroll wheel",
			Cause:   err,
		}
	}

	var docGen uint64
	if i.tabMgr != nil {
		if rec, ok := i.tabMgr.store.get(tabID); ok {
			docGen = rec.documentGeneration
		}
	}

	return &BrowserInteractionResult{
		Success:            true,
		Action:             "scroll",
		Strategy:           "cdp_wheel",
		Verified:           true,
		DocumentGeneration: docGen,
		DurationMS:         time.Since(startTime).Milliseconds(),
	}, nil
}

func pickClickPoint(quads [][2]float64) (float64, float64) {
	if len(quads) < 4 {
		return -1, -1
	}

	minX := quads[0][0]
	minY := quads[0][1]
	maxX := quads[0][0]
	maxY := quads[0][1]

	for _, q := range quads {
		if q[0] < minX {
			minX = q[0]
		}
		if q[0] > maxX {
			maxX = q[0]
		}
		if q[1] < minY {
			minY = q[1]
		}
		if q[1] > maxY {
			maxY = q[1]
		}
	}

	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2

	if centerX < 0 || centerY < 0 {
		return -1, -1
	}

	return centerX, centerY
}
