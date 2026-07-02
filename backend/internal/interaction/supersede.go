package interaction

import (
	"context"
	"errors"
	"sync"
)

type SupersedePolicy string

const (
	SupersedePolicyLatest SupersedePolicy = "latest"
	SupersedePolicyQueue  SupersedePolicy = "queue"
)

var (
	ErrSupersedeNoActiveInteraction = errors.New("supersede: no active interaction to supersede")
)

type SupersedeResolution struct {
	SupersedeTargetID string
	Enqueue           bool
	RejectNew         bool
}

type SupersedeResolver struct {
	policy  SupersedePolicy
	tracker InteractionTracker
	mu      sync.RWMutex
}

func NewSupersedeResolver(policy SupersedePolicy, tracker InteractionTracker) *SupersedeResolver {
	return &SupersedeResolver{
		policy:  policy,
		tracker: tracker,
	}
}

func (r *SupersedeResolver) Resolve(ctx context.Context, scope InteractionScope) (*SupersedeResolution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	active, err := r.tracker.ListActive(ctx, scope)
	if err != nil {
		return nil, err
	}
	if len(active) == 0 {
		return &SupersedeResolution{}, nil
	}

	switch r.policy {
	case SupersedePolicyLatest:
		return r.resolveLatest(active)
	case SupersedePolicyQueue:
		return r.resolveQueue(active)
	default:
		return r.resolveLatest(active)
	}
}

func (r *SupersedeResolver) resolveLatest(active []*InteractionRecord) (*SupersedeResolution, error) {
	target := active[0]
	for _, rec := range active[1:] {
		if rec.CreatedAt.After(target.CreatedAt) {
			target = rec
		}
	}
	return &SupersedeResolution{
		SupersedeTargetID: target.ID,
		Enqueue:           false,
		RejectNew:         false,
	}, nil
}

func (r *SupersedeResolver) resolveQueue(active []*InteractionRecord) (*SupersedeResolution, error) {
	const maxQueueDepth = 10
	if len(active) >= maxQueueDepth {
		return &SupersedeResolution{
			Enqueue:   false,
			RejectNew: true,
		}, nil
	}
	return &SupersedeResolution{
		Enqueue: true,
	}, nil
}
