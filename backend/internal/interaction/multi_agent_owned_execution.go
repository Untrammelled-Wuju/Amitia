package interaction

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedCoordinationAuthorityKey struct{}

func (c *MultiAgentCoordinator) SetOwnedExecution(run func(context.Context, coordination.ExecutionScope, func(context.Context) error) error) {
	c.mu.Lock()
	c.ownedExecution = run
	c.mu.Unlock()
}

func (c *MultiAgentCoordinator) withOwnedExecution(ctx context.Context, ac *activeCoordination, run func(context.Context) error) error {
	if ac.ownedScope == nil {
		return run(ctx)
	}
	expected := *ac.ownedScope
	if authorized, ok := ctx.Value(ownedCoordinationAuthorityKey{}).(coordination.ExecutionScope); ok && authorized == expected {
		return run(ctx)
	}
	c.mu.Lock()
	factory := c.ownedExecution
	c.mu.Unlock()
	if factory == nil {
		return errors.New("multi_agent: owned execution authority unavailable")
	}
	return factory(ctx, expected, func(ctx context.Context) error {
		actual, ok := coordination.FromContext(ctx)
		if !ok || actual != expected {
			return coordination.ErrScopeExpired
		}
		return run(context.WithValue(ctx, ownedCoordinationAuthorityKey{}, expected))
	})
}

func (c *MultiAgentCoordinator) parentCancellationRequested(ctx context.Context, ac *activeCoordination) (bool, error) {
	if ac.parentInteractionID == "" || c.tracker == nil {
		return false, nil
	}
	parent, found, err := c.tracker.Get(ctx, ac.parentInteractionID)
	if err != nil {
		return false, err
	}
	if !found || parent == nil {
		return false, ErrInteractionNotFound
	}
	return !parent.CancelRequestedAt.IsZero() || parent.Status == InteractionStatusCancelled || parent.Status == InteractionStatusSuperseded || parent.Status == InteractionStatusInterrupted, nil
}

func (c *MultiAgentCoordinator) requireDispatchableParent(ctx context.Context, ac *activeCoordination) error {
	cancelled, err := c.parentCancellationRequested(ctx, ac)
	if err != nil {
		return err
	}
	if cancelled {
		return errors.New("multi_agent: parent interaction cancellation prohibits worker dispatch")
	}
	return nil
}

func (c *MultiAgentCoordinator) stopRevokedOwnedCoordination(ctx context.Context, ac *activeCoordination, cause error) error {
	if ac.ownedScope == nil || !(errors.Is(cause, coordination.ErrScopeExpired) || errors.Is(cause, coordination.ErrRoleRequired) || errors.Is(cause, coordination.ErrCapabilityGrant) || errors.Is(cause, coordination.ErrWrongOwner)) {
		return cause
	}
	stopContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	c.mu.Lock()
	for _, assignment := range ac.assignments {
		if !assignment.Status.IsTerminal() {
			assignment.Error = "authority_revoked: " + cause.Error()
		}
	}
	c.mu.Unlock()
	return errors.Join(cause, c.Cancel(stopContext, string(ac.id)))
}
