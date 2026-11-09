package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type RuntimeMaintenanceProvider interface {
	Overview(context.Context) (any, error)
	Status(context.Context) (any, error)
	Config(context.Context) (any, error)
	UpdateConfig(context.Context, json.RawMessage) (any, error)
	CleanupTemp(context.Context) (any, error)
	RotateLogs(context.Context) (any, error)
	CheckDBIntegrity(context.Context) (any, error)
}

type RuntimeMaintenanceBridge struct {
	mu       sync.RWMutex
	provider RuntimeMaintenanceProvider
}

func NewRuntimeMaintenanceBridge() *RuntimeMaintenanceBridge {
	return &RuntimeMaintenanceBridge{}
}

func (b *RuntimeMaintenanceBridge) SetProvider(provider RuntimeMaintenanceProvider) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.provider = provider
	b.mu.Unlock()
}

func (b *RuntimeMaintenanceBridge) Status(ctx context.Context) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.Status(ctx)
	})
}

func (b *RuntimeMaintenanceBridge) Overview(ctx context.Context) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.Overview(ctx)
	})
}

func (b *RuntimeMaintenanceBridge) Config(ctx context.Context) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.Config(ctx)
	})
}

func (b *RuntimeMaintenanceBridge) UpdateConfig(ctx context.Context, input json.RawMessage) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.UpdateConfig(ctx, input)
	})
}

func (b *RuntimeMaintenanceBridge) CleanupTemp(ctx context.Context) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.CleanupTemp(ctx)
	})
}

func (b *RuntimeMaintenanceBridge) RotateLogs(ctx context.Context) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.RotateLogs(ctx)
	})
}

func (b *RuntimeMaintenanceBridge) CheckDBIntegrity(ctx context.Context) (any, error) {
	return b.call(ctx, func(provider RuntimeMaintenanceProvider) (any, error) {
		return provider.CheckDBIntegrity(ctx)
	})
}

func (b *RuntimeMaintenanceBridge) call(
	_ context.Context,
	fn func(RuntimeMaintenanceProvider) (any, error),
) (any, error) {
	if b == nil {
		return nil, fmt.Errorf("runtime maintenance service is unavailable")
	}
	b.mu.RLock()
	provider := b.provider
	b.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("runtime maintenance service is unavailable")
	}
	return fn(provider)
}
