package interaction

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/decision"
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
	if parent.Status == InteractionStatusPausing || parent.Status == InteractionStatusPaused {
		return false, errors.New("multi_agent: parent interaction is paused; worker execution is not authorized")
	}
	return !parent.CancelRequestedAt.IsZero() || parent.IsTerminal(), nil
}

func (c *MultiAgentCoordinator) requireDispatchableParent(ctx context.Context, ac *activeCoordination) error {
	if c == nil || c.goals == nil || ac == nil {
		return errors.New("multi_agent: parent goal registry unavailable")
	}
	goal, found := c.goals.Get(ac.parentGoalID)
	if !found || goal.Revision != ac.parentGoalRev {
		return errors.New("multi_agent: parent goal missing or revision changed")
	}
	if err := validateDispatchableGoalStatus(goal.Status); err != nil {
		return err
	}
	if err := validateCoordinationTimeBudget(c.clock.Now(), goal.ExpiresAt, ac.deadlineAt); err != nil {
		return err
	}
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

func validateDispatchableGoalStatus(status decision.GoalStatus) error {
	if status != decision.GoalStatusActive && status != decision.GoalStatusPending {
		return errors.New("multi_agent: parent goal is not active for worker dispatch: " + string(status))
	}
	return nil
}
