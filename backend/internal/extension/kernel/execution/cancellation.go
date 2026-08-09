package execution

import (
	"context"
	"errors"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type cancellationState uint8

const (
	cancellationStateActive cancellationState = iota
	cancellationStateRequested
	cancellationStateTerminal
)

type invocationCancelledError struct {
	reason capability.ToolCancellationReason
}

func (e *invocationCancelledError) Error() string {
	return "tool invocation cancelled"
}

type runtimeCancelFunc func()

type activeCancellation struct {
	Invocation     capability.ToolInvocationContext
	cancel         context.CancelCauseFunc
	state          cancellationState
	reason         capability.ToolCancellationReason
	runtimeCancels []runtimeCancelFunc
	attached       bool
}

func NewCancellationController() *CancellationController {
	return &CancellationController{
		active:   make(map[string]*activeCancellation),
		children: make(map[string]map[string]struct{}),
		roots:    make(map[string]map[string]struct{}),
		external: make(map[string]string),
	}
}

type CancellationController struct {
	mu       sync.Mutex
	active   map[string]*activeCancellation
	children map[string]map[string]struct{}
	roots    map[string]map[string]struct{}
	external map[string]string
}

type CancellationResult struct {
	Requested              bool
	TargetInvocationID     string
	CancelledInvocationIDs []string
	AlreadyRequested       []string
}

func (c *CancellationController) Register(ctx context.Context, inv capability.ToolInvocationContext) (context.Context, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	invID := inv.InvocationID
	if _, exists := c.active[invID]; exists {
		return ctx, func() {}, errors.New("duplicate active invocation ID: " + invID)
	}

	runCtx, cancel := context.WithCancelCause(ctx)

	ac := &activeCancellation{
		Invocation: inv,
		cancel:     cancel,
		state:      cancellationStateActive,
	}

	c.active[invID] = ac

	if inv.ParentID != "" {
		if c.children[inv.ParentID] == nil {
			c.children[inv.ParentID] = make(map[string]struct{})
		}
		c.children[inv.ParentID][invID] = struct{}{}
	}

	if inv.RootID != "" {
		if c.roots[inv.RootID] == nil {
			c.roots[inv.RootID] = make(map[string]struct{})
		}
		c.roots[inv.RootID][invID] = struct{}{}
	}

	if inv.ExternalCallID != "" {
		scope := capability.CancellationExternalScope{
			UserID:         inv.UserID,
			CharacterID:    inv.CharacterID,
			ConversationID: inv.ConversationID,
			SessionID:      inv.SessionID,
		}
		extKey := scope.Key() + "|" + inv.ExternalCallID
		if _, exists := c.external[extKey]; exists {
			cancel(&invocationCancelledError{})
			delete(c.active, invID)
			if inv.ParentID != "" {
				deleteParentLink(c.children, inv.ParentID, invID)
			}
			if inv.RootID != "" {
				delete(c.roots[inv.RootID], invID)
			}
			return ctx, func() {}, errors.New("duplicate external call ID: " + inv.ExternalCallID)
		}
		c.external[extKey] = invID
	}

	cleanup := func() {
		c.cleanup(invID)
	}

	return runCtx, cleanup, nil
}

func (c *CancellationController) cleanup(invID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ac, exists := c.active[invID]
	if !exists {
		return
	}

	if ac.Invocation.ParentID != "" {
		deleteParentLink(c.children, ac.Invocation.ParentID, invID)
	}

	if ac.Invocation.RootID != "" {
		if rootSet, ok := c.roots[ac.Invocation.RootID]; ok {
			delete(rootSet, invID)
			if len(rootSet) == 0 {
				delete(c.roots, ac.Invocation.RootID)
			}
		}
	}

	if ac.Invocation.ExternalCallID != "" {
		scope := capability.CancellationExternalScope{
			UserID:         ac.Invocation.UserID,
			CharacterID:    ac.Invocation.CharacterID,
			ConversationID: ac.Invocation.ConversationID,
			SessionID:      ac.Invocation.SessionID,
		}
		extKey := scope.Key() + "|" + ac.Invocation.ExternalCallID
		delete(c.external, extKey)
	}

	delete(c.active, invID)
}

func deleteParentLink(children map[string]map[string]struct{}, parentID, childID string) {
	if childSet, ok := children[parentID]; ok {
		delete(childSet, childID)
		if len(childSet) == 0 {
			delete(children, parentID)
		}
	}
}

func (c *CancellationController) CancelInvocation(ctx context.Context, invocationID string, reason capability.ToolCancellationReason) CancellationResult {
	if !reason.Code.Valid() {
		reason = capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	}

	c.mu.Lock()

	target, exists := c.active[invocationID]
	if !exists || target.state == cancellationStateTerminal {
		c.mu.Unlock()
		return CancellationResult{Requested: false, TargetInvocationID: invocationID}
	}

	idsToCancel := make([]string, 0)
	alreadyRequested := make([]string, 0)

	var collectDescendants func(rootID string, rootAc *activeCancellation)
	collectDescendants = func(rootID string, rootAc *activeCancellation) {
		if rootAc.state == cancellationStateTerminal {
			return
		}
		if rootAc.state == cancellationStateRequested {
			alreadyRequested = append(alreadyRequested, rootID)
			return
		}
		idsToCancel = append(idsToCancel, rootID)
		for childID := range c.children[rootID] {
			if childAc, ok := c.active[childID]; ok {
				collectDescendants(childID, childAc)
			}
		}
	}

	collectDescendants(invocationID, target)

	if len(idsToCancel) == 0 {
		c.mu.Unlock()
		return CancellationResult{
			Requested:          false,
			TargetInvocationID: invocationID,
			AlreadyRequested:   alreadyRequested,
		}
	}

	cancelledIDs := make([]string, 0, len(idsToCancel))
	cancelFuncs := make([]func(), 0, len(idsToCancel))
	runtimeCancelLists := make([][]runtimeCancelFunc, 0, len(idsToCancel))

	for _, id := range idsToCancel {
		ac := c.active[id]
		ac.state = cancellationStateRequested
		childReason := reason
		if id != invocationID {
			childReason = capability.ToolCancellationReason{
				Code:               capability.CancellationReasonParentCancelled,
				OriginInvocationID: invocationID,
			}
		}
		ac.reason = childReason
		cancelFuncs = append(cancelFuncs, func() {
			ac.cancel(&invocationCancelledError{reason: childReason})
		})
		if len(ac.runtimeCancels) > 0 {
			runtimeCancelLists = append(runtimeCancelLists, ac.runtimeCancels)
		}
		cancelledIDs = append(cancelledIDs, id)
	}

	c.mu.Unlock()

	for _, fn := range cancelFuncs {
		fn()
	}
	for _, rcList := range runtimeCancelLists {
		for _, rc := range rcList {
			rc()
		}
	}

	return CancellationResult{
		Requested:              true,
		TargetInvocationID:     invocationID,
		CancelledInvocationIDs: cancelledIDs,
		AlreadyRequested:       alreadyRequested,
	}
}

func (c *CancellationController) CancelRoot(ctx context.Context, rootID string, reason capability.ToolCancellationReason) CancellationResult {
	if !reason.Code.Valid() {
		reason = capability.ToolCancellationReason{Code: capability.CancellationReasonUserRequested}
	}

	c.mu.Lock()

	rootSet, exists := c.roots[rootID]
	if !exists || len(rootSet) == 0 {
		c.mu.Unlock()
		return CancellationResult{Requested: false}
	}

	invocationIDs := make([]string, 0, len(rootSet))
	for id := range rootSet {
		invocationIDs = append(invocationIDs, id)
	}

	c.mu.Unlock()

	allCancelled := make([]string, 0)
	allAlreadyRequested := make([]string, 0)
	anyRequested := false

	for _, id := range invocationIDs {
		result := c.CancelInvocation(ctx, id, reason)
		if result.Requested {
			anyRequested = true
			allCancelled = append(allCancelled, result.CancelledInvocationIDs...)
		}
		allAlreadyRequested = append(allAlreadyRequested, result.AlreadyRequested...)
	}

	return CancellationResult{
		Requested:              anyRequested,
		CancelledInvocationIDs: allCancelled,
		AlreadyRequested:       allAlreadyRequested,
	}
}

func (c *CancellationController) CancelExternalCall(ctx context.Context, scope capability.CancellationExternalScope, externalCallID string, reason capability.ToolCancellationReason) CancellationResult {
	extKey := scope.Key() + "|" + externalCallID

	c.mu.Lock()
	invID, exists := c.external[extKey]
	c.mu.Unlock()

	if !exists {
		return CancellationResult{Requested: false}
	}

	return c.CancelInvocation(ctx, invID, reason)
}

func (c *CancellationController) AttachRuntimeCanceller(invocationID string, fn runtimeCancelFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ac, exists := c.active[invocationID]
	if !exists {
		return errors.New("invocation not active: " + invocationID)
	}

	ac.runtimeCancels = append(ac.runtimeCancels, fn)
	ac.attached = true

	if ac.state == cancellationStateRequested {
		childReason := capability.ToolCancellationReason{
			Code:               capability.CancellationReasonParentCancelled,
			OriginInvocationID: invocationID,
		}
		c.mu.Unlock()
		fn()
		ac.cancel(&invocationCancelledError{reason: childReason})
		c.mu.Lock()
	}

	return nil
}

func (c *CancellationController) RequestRuntimeAbort(ctx context.Context, invocationID string, reason capability.ToolCancellationReason) (bool, error) {
	c.mu.Lock()
	ac, exists := c.active[invocationID]
	if !exists || len(ac.runtimeCancels) == 0 {
		c.mu.Unlock()
		return false, nil
	}
	runtimeCancels := make([]runtimeCancelFunc, len(ac.runtimeCancels))
	copy(runtimeCancels, ac.runtimeCancels)
	c.mu.Unlock()

	for _, rc := range runtimeCancels {
		rc()
	}

	return true, nil
}

func (c *CancellationController) Finalize(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	c.mu.Lock()

	ac, exists := c.active[inv.InvocationID]
	if !exists || ac.state == cancellationStateTerminal {
		c.mu.Unlock()
		return result
	}

	if ac.state == cancellationStateRequested {
		ac.state = cancellationStateTerminal
		c.mu.Unlock()
		return c.buildCancelledResult(inv, ac.reason, result)
	}

	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			ac.state = cancellationStateTerminal
			c.mu.Unlock()
			return c.buildTimedOutResult(inv, result)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			canceledErr := context.Cause(ctx)
			var cancelledErr *invocationCancelledError
			if errors.As(canceledErr, &cancelledErr) {
				ac.reason = cancelledErr.reason
			} else {
				ac.reason = capability.ToolCancellationReason{Code: capability.CancellationReasonCallerContext}
			}
			ac.state = cancellationStateTerminal
			c.mu.Unlock()
			return c.buildCancelledResult(inv, ac.reason, result)
		}
	}

	ac.state = cancellationStateTerminal
	c.mu.Unlock()
	return result
}

func (c *CancellationController) buildTimedOutResult(inv capability.ToolInvocationContext, previous capability.UnifiedToolResult) capability.UnifiedToolResult {
	timedOutErr := &capability.ToolError{
		Code:        capability.ErrorCodeTimeout,
		Category:    capability.ToolErrorCategoryTimeout,
		Message:     "tool execution timed out",
		Retryable:   false,
		UserVisible: false,
	}

	timedOutResult := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		Status:       capability.ToolResultStatusTimedOut,
		Error:        timedOutErr,
		SideEffects:  previous.SideEffects,
		DurationMS:   previous.DurationMS,
	}

	return timedOutResult
}

func (c *CancellationController) buildCancelledResult(inv capability.ToolInvocationContext, reason capability.ToolCancellationReason, previous capability.UnifiedToolResult) capability.UnifiedToolResult {
	cancelledErr := &capability.ToolError{
		Code:        capability.ErrorCodeCancelled,
		Category:    capability.ToolErrorCategoryCancellation,
		Message:     "tool execution cancelled",
		Retryable:   false,
		UserVisible: false,
		Details: map[string]any{
			"reasonCode": string(reason.Code),
		},
	}

	if reason.OriginInvocationID != "" {
		cancelledErr.Details["originInvocationId"] = reason.OriginInvocationID
	}

	cancelledResult := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		Status:       capability.ToolResultStatusCancelled,
		Error:        cancelledErr,
		SideEffects:  previous.SideEffects,
		DurationMS:   previous.DurationMS,
	}

	return cancelledResult
}

func (c *CancellationController) ResolveCancellation(ctx context.Context, inv capability.ToolInvocationContext) (capability.UnifiedToolResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ac, exists := c.active[inv.InvocationID]
	if !exists {
		return capability.UnifiedToolResult{}, false
	}

	if ac.state == cancellationStateRequested {
		result := c.buildCancelledResult(inv, ac.reason, capability.UnifiedToolResult{InvocationID: inv.InvocationID})
		return result, true
	}

	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		canceledErr := context.Cause(ctx)
		var cancelReason capability.ToolCancellationReason
		var cancelledErr *invocationCancelledError
		if errors.As(canceledErr, &cancelledErr) {
			cancelReason = cancelledErr.reason
		} else {
			cancelReason = capability.ToolCancellationReason{Code: capability.CancellationReasonCallerContext}
		}
		result := c.buildCancelledResult(inv, cancelReason, capability.UnifiedToolResult{InvocationID: inv.InvocationID})
		return result, true
	}

	return capability.UnifiedToolResult{}, false
}

func (c *CancellationController) IsActive(invocationID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ac, exists := c.active[invocationID]
	if !exists {
		return false
	}
	return ac.state != cancellationStateTerminal
}

func (c *CancellationController) CancelReason(invocationID string) (capability.ToolCancellationReason, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ac, exists := c.active[invocationID]
	if !exists || ac.state != cancellationStateRequested {
		return capability.ToolCancellationReason{}, false
	}
	return ac.reason, true
}
