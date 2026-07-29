package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type HandlerFunc func(ctx context.Context, input HookInvocationInput) (HookResult, error)

type DirectRuntimeBridge struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

func NewDirectRuntimeBridge() *DirectRuntimeBridge {
	return &DirectRuntimeBridge{
		handlers: make(map[string]HandlerFunc),
	}
}

func (b *DirectRuntimeBridge) Bind(contributionID string, handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[contributionID] = handler
}

func (b *DirectRuntimeBridge) Unbind(contributionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handlers, contributionID)
}

func (b *DirectRuntimeBridge) Invoke(ctx context.Context, contrib HookContributionDefinition, input HookInvocationInput) (HookResult, error) {
	b.mu.RLock()
	handler, ok := b.handlers[contrib.ContributionID]
	b.mu.RUnlock()
	if !ok {
		return HookResult{}, NewHookError(ErrCodeRuntimeNotReady, "no handler bound for contribution: "+contrib.ContributionID)
	}
	return handler(ctx, input)
}

func (b *DirectRuntimeBridge) IsReady(_ context.Context, contrib HookContributionDefinition) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.handlers[contrib.ContributionID]
	return ok
}

type SupervisorRuntimeBridge struct {
	Supervisor       runtime_supervisor.Supervisor
	InstanceResolver func(ctx context.Context, contrib HookContributionDefinition) (string, error)
	mu               sync.RWMutex
	instanceCache    map[string]string
}

func NewSupervisorRuntimeBridge(supervisor runtime_supervisor.Supervisor, resolver func(ctx context.Context, contrib HookContributionDefinition) (string, error)) *SupervisorRuntimeBridge {
	return &SupervisorRuntimeBridge{
		Supervisor:       supervisor,
		InstanceResolver: resolver,
		instanceCache:    make(map[string]string),
	}
}

func (b *SupervisorRuntimeBridge) resolveInstance(ctx context.Context, contrib HookContributionDefinition) (string, error) {
	b.mu.RLock()
	if id, ok := b.instanceCache[contrib.ContributionID]; ok {
		b.mu.RUnlock()
		return id, nil
	}
	b.mu.RUnlock()

	if b.InstanceResolver == nil {
		return "", fmt.Errorf("hook: no instance resolver configured")
	}

	instanceID, err := b.InstanceResolver(ctx, contrib)
	if err != nil {
		return "", err
	}

	b.mu.Lock()
	b.instanceCache[contrib.ContributionID] = instanceID
	b.mu.Unlock()
	return instanceID, nil
}

func (b *SupervisorRuntimeBridge) Invoke(ctx context.Context, contrib HookContributionDefinition, input HookInvocationInput) (HookResult, error) {
	instanceID, err := b.resolveInstance(ctx, contrib)
	if err != nil {
		return HookResult{}, NewHookError(ErrCodeRuntimeNotReady, "resolve instance: "+err.Error())
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return HookResult{}, NewHookError(ErrCodeHookResultInvalid, "marshal input: "+err.Error())
	}

	invReq := runtime_supervisor.InvocationRequest{
		InstanceID:   instanceID,
		InvocationID: input.Context.InvocationID,
		TraceID:      input.Context.TraceID,
		Operation:    "hook:" + contrib.HookPointID + ":" + contrib.Entry,
		Input:        inputBytes,
	}

	result := b.Supervisor.Invoke(ctx, invReq)
	if result.Error != nil {
		return HookResult{}, NewHookError(ErrCodeHookRuntimeError, result.Error.Error())
	}

	var hookResult HookResult
	if err := json.Unmarshal(result.Output, &hookResult); err != nil {
		return HookResult{}, NewHookError(ErrCodeHookResultInvalid, "unmarshal result: "+err.Error())
	}

	return hookResult, nil
}

func (b *SupervisorRuntimeBridge) IsReady(ctx context.Context, contrib HookContributionDefinition) bool {
	instanceID, err := b.resolveInstance(ctx, contrib)
	if err != nil {
		return false
	}
	_, err = b.Supervisor.GetInstance(ctx, instanceID)
	return err == nil
}

func (b *SupervisorRuntimeBridge) ClearCache(contributionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.instanceCache, contributionID)
}

func (b *SupervisorRuntimeBridge) ClearAllCache() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.instanceCache = make(map[string]string)
}
