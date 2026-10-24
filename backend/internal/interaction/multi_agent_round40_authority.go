package interaction

import (
	"context"
	"fmt"
)

func (c *MultiAgentCoordinator) validateCancellationChild(ctx context.Context, ac *activeCoordination, child AgentAssignment) error {
	if c == nil || ac == nil || child.ChildInteractionID == "" || child.ID == "" {
		return fmt.Errorf("missing durable child interaction for cancellation")
	}
	if c.tracker == nil {
		if ac.ownedScope != nil {
			return fmt.Errorf("owned cancellation requires a durable interaction tracker")
		}
		return nil
	}
	if ac.ownedScope != nil {
		reserver, ok := c.starter.(interface{ WorkerInteractionID(WorkerRunRequest) string })
		if !ok || reserver.WorkerInteractionID(WorkerRunRequest{AssignmentID: child.ID}) != child.ChildInteractionID {
			return fmt.Errorf("owned worker cancellation identity cannot be verified")
		}
	}
	record, found, err := c.tracker.Get(ctx, child.ChildInteractionID)
	if err != nil {
		return err
	}
	if !found || record == nil {
		if ac.ownedScope != nil {
			return fmt.Errorf("owned worker reservation missing; reconcile before cancellation")
		}
		return nil
	}
	spaceID, conversationID, err := c.cancellationScope(ctx, ac)
	if err != nil {
		return err
	}
	return validateReconciledChildScope(record, child, spaceID, conversationID)
}

func (c *MultiAgentCoordinator) validateOwnedTerminalResult(ctx context.Context, ac *activeCoordination, assignment AgentAssignment, result AgentAssignmentResult) error {
	if c.tracker == nil {
		return fmt.Errorf("owned worker terminal proof requires an interaction tracker")
	}
	child, found, err := c.tracker.Get(ctx, assignment.ChildInteractionID)
	if err != nil {
		return err
	}
	if !found || child == nil {
		return fmt.Errorf("owned worker terminal proof is missing")
	}
	spaceID, conversationID, err := c.resolveWorkerScope(ctx, ac)
	if err != nil {
		return err
	}
	if err := validateReconciledChildScope(child, assignment, spaceID, conversationID); err != nil {
		return err
	}
	return validateTerminalChildSnapshot(child, CoordinationAssignmentSnapshot{
		AssignmentID: assignment.ID, ChildInteractionID: assignment.ChildInteractionID, Status: result.Status,
	})
}

func (c *MultiAgentCoordinator) cancellationScope(ctx context.Context, ac *activeCoordination) (string, string, error) {
	if ac.parentInteractionID == "" {
		if ac.ownedScope != nil {
			return "", "", fmt.Errorf("owned cancellation requires a durable parent")
		}
		return "", "", nil
	}
	if c.tracker == nil {
		return "", "", fmt.Errorf("parent tracker unavailable during cancellation")
	}
	parent, found, err := c.tracker.Get(ctx, ac.parentInteractionID)
	if err != nil {
		return "", "", err
	}
	if !found || parent == nil {
		return "", "", ErrInteractionNotFound
	}
	scope := parent.Scope.Normalize()
	if ac.ownedScope != nil && (scope.SpaceID == "" || scope.ConversationID == "" || scope.SpaceID != ac.ownedScope.SpaceID || scope.CharacterID != ac.ownedScope.RoleID) {
		return "", "", fmt.Errorf("owned cancellation parent scope mismatch")
	}
	return scope.SpaceID, scope.ConversationID, nil
}
