package hook

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type HookPointRegistry interface {
	RegisterPoint(ctx context.Context, point HookPointDefinition) error
	GetPoint(ctx context.Context, hookPointID string) (HookPointDefinition, error)
	ListPoints(ctx context.Context) ([]HookPointDefinition, error)
	ListThirdPartyPoints(ctx context.Context) ([]HookPointDefinition, error)
}

type DefaultHookPointRegistry struct {
	mu     sync.RWMutex
	points map[string]HookPointDefinition
}

func NewHookPointRegistry() *DefaultHookPointRegistry {
	return &DefaultHookPointRegistry{
		points: make(map[string]HookPointDefinition),
	}
}

func (r *DefaultHookPointRegistry) RegisterPoint(_ context.Context, point HookPointDefinition) error {
	if point.HookPointID == "" {
		return NewHookError(ErrCodeHookResultInvalid, "hook point id required")
	}
	if point.ContractVersion < 1 {
		return NewHookError(ErrCodeHookResultInvalid, "contract version must be >= 1")
	}
	if len(point.SupportedPhases) == 0 {
		return NewHookError(ErrCodeHookResultInvalid, "supported phases required")
	}
	if point.MaxHandlers <= 0 {
		point.MaxHandlers = 16
	}
	if point.DefaultTimeout <= 0 {
		point.DefaultTimeout = 500_000_000
	}
	if point.MaxTimeout <= 0 {
		point.MaxTimeout = point.DefaultTimeout
	}
	if point.MaxPayloadBytes <= 0 {
		point.MaxPayloadBytes = 256 * 1024
	}
	if point.MaxResultBytes <= 0 {
		point.MaxResultBytes = 128 * 1024
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.points[point.HookPointID]; exists {
		return ErrPointExists
	}
	r.points[point.HookPointID] = point
	return nil
}

func (r *DefaultHookPointRegistry) GetPoint(_ context.Context, hookPointID string) (HookPointDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.points[hookPointID]
	if !ok {
		return HookPointDefinition{}, fmt.Errorf("%w: %s", ErrPointNotFound, hookPointID)
	}
	return p, nil
}

func (r *DefaultHookPointRegistry) ListPoints(_ context.Context) ([]HookPointDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]HookPointDefinition, 0, len(r.points))
	for _, p := range r.points {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].HookPointID < out[j].HookPointID
	})
	return out, nil
}

func (r *DefaultHookPointRegistry) ListThirdPartyPoints(ctx context.Context) ([]HookPointDefinition, error) {
	all, err := r.ListPoints(ctx)
	if err != nil {
		return nil, err
	}
	var out []HookPointDefinition
	for _, p := range all {
		if p.ThirdPartyAllowed {
			out = append(out, p)
		}
	}
	return out, nil
}
