package acquisition

import (
	"context"
	"sync"
	"time"
)

// AcquisitionEventKind enumerates the types of events that can be dispatched
// through the hook system.
type AcquisitionEventKind string

const (
	EventAcquisitionStarted   AcquisitionEventKind = "acquisition.started"
	EventAcquisitionCompleted AcquisitionEventKind = "acquisition.completed"
	EventAcquisitionFailed    AcquisitionEventKind = "acquisition.failed"
	EventApprovalRequired     AcquisitionEventKind = "approval.required"
	EventPolicyDecision       AcquisitionEventKind = "policy.decision"
)

// AcquisitionEvent is the payload dispatched to hooks when an acquisition
// lifecycle event occurs.
type AcquisitionEvent struct {
	Kind        AcquisitionEventKind `json:"kind"`
	Request     *AcquisitionRequest `json:"request,omitempty"`
	CandidateID string              `json:"candidateId,omitempty"`
	Result      *AcquisitionResult  `json:"result,omitempty"`
	Error       string              `json:"error,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
}

// AcquisitionHook is the interface that all acquisition lifecycle hooks must
// implement. Hooks are invoked synchronously; they should not block for
// extended periods.
type AcquisitionHook interface {
	OnAcquisitionEvent(ctx context.Context, event AcquisitionEvent) error
}

// AcquisitionHookChain aggregates multiple hooks and dispatches events to each
// in registration order. All hooks are invoked even when some return errors.
type AcquisitionHookChain struct {
	mu    sync.RWMutex
	hooks []AcquisitionHook
}

// NewAcquisitionHookChain returns an AcquisitionHookChain pre-populated with the
// provided hooks.
func NewAcquisitionHookChain(hooks ...AcquisitionHook) *AcquisitionHookChain {
	return &AcquisitionHookChain{
		hooks: append([]AcquisitionHook(nil), hooks...),
	}
}

// Append adds a new hook to the end of the chain.
func (c *AcquisitionHookChain) Append(h AcquisitionHook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, h)
}

// Dispatch invokes OnAcquisitionEvent on every hook in the chain. Errors from
// individual hooks are collected and returned as a slice. Hooks are called in
// registration order.
func (c *AcquisitionHookChain) Dispatch(ctx context.Context, event AcquisitionEvent) []error {
	c.mu.RLock()
	hooks := append([]AcquisitionHook(nil), c.hooks...)
	c.mu.RUnlock()

	var errs []error
	for _, h := range hooks {
		if err := h.OnAcquisitionEvent(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
