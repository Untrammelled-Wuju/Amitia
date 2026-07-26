package execution

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewSideEffectRecorder() *SideEffectRecorder {
	return &SideEffectRecorder{
		records: make(map[string][]capability.RecordedSideEffect),
	}
}

type SideEffectRecorder struct {
	records map[string][]capability.RecordedSideEffect
	mu      sync.RWMutex
}

func (r *SideEffectRecorder) Record(ctx context.Context, invocationID, toolID string, effects []capability.RecordedSideEffect) {
	if len(effects) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	tagged := make([]capability.RecordedSideEffect, 0, len(effects))
	for _, e := range effects {
		if e.Type == "" {
			e.Type = "unknown"
		}
		tagged = append(tagged, e)
	}
	r.records[invocationID] = tagged
}

func (r *SideEffectRecorder) GetEffects(invocationID string) []capability.RecordedSideEffect {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.records[invocationID]
}

func (r *SideEffectRecorder) Count(invocationID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.records[invocationID])
}

func (r *SideEffectRecorder) PruneOlderThan(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
}
