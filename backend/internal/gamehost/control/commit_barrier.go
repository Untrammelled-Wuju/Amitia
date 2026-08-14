package control

import (
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ControlCommitBarrierImpl struct {
	mu       sync.Mutex
	barriers map[domain.RuntimeInstanceID]*runtimeBarrier
}

type runtimeBarrier struct {
	mu       sync.RWMutex
}

func NewControlCommitBarrier() *ControlCommitBarrierImpl {
	return &ControlCommitBarrierImpl{
		barriers: make(map[domain.RuntimeInstanceID]*runtimeBarrier),
	}
}

func (b *ControlCommitBarrierImpl) getBarrier(runtimeID domain.RuntimeInstanceID) *runtimeBarrier {
	b.mu.Lock()
	defer b.mu.Unlock()
	rb, ok := b.barriers[runtimeID]
	if !ok {
		rb = &runtimeBarrier{}
		b.barriers[runtimeID] = rb
	}
	return rb
}

func (b *ControlCommitBarrierImpl) WithReadCommit(runtimeID domain.RuntimeInstanceID, fn func()) error {
	if fn == nil {
		return fmt.Errorf("commit barrier: fn is nil")
	}
	rb := b.getBarrier(runtimeID)
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	fn()
	return nil
}

func (b *ControlCommitBarrierImpl) WithExclusiveMutation(runtimeID domain.RuntimeInstanceID, fn func()) error {
	if fn == nil {
		return fmt.Errorf("commit barrier: fn is nil")
	}
	rb := b.getBarrier(runtimeID)
	rb.mu.Lock()
	defer rb.mu.Unlock()
	fn()
	return nil
}

func (b *ControlCommitBarrierImpl) RemoveBarrier(runtimeID domain.RuntimeInstanceID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.barriers, runtimeID)
}

var _ ControlCommitBarrier = (*ControlCommitBarrierImpl)(nil)
