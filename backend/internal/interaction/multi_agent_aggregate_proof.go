package interaction

import (
	"context"
	"fmt"
	"strings"
)

func (c *MultiAgentCoordinator) verifyOwnedAggregateEvidence(ctx context.Context, ac *activeCoordination) error {
	if ac == nil || ac.ownedScope == nil {
		return nil
	}
	if c.tracker == nil {
		return fmt.Errorf("multi_agent: cannot aggregate owned children without interaction tracker")
	}
	c.mu.Lock()
	assignments := make([]AgentAssignment, 0, len(ac.assignments))
	for _, item := range ac.assignments {
		assignments = append(assignments, *item)
	}
	cancelRequested := ac.cancelRequested
	c.mu.Unlock()

	spaceID, conversationID, err := c.resolveWorkerScope(ctx, ac)
	if err != nil {
		return err
	}
	if spaceID == "" || conversationID == "" {
		return fmt.Errorf("multi_agent: owned aggregate has no durable worker scope")
	}
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !assignment.Status.IsTerminal() {
			return fmt.Errorf("multi_agent: cannot finalize owned parent while assignment %s remains %s", assignment.ID, assignment.Status)
		}
		if assignment.ChildInteractionID == "" {
			if assignment.Status == AssignmentFailed && strings.HasPrefix(assignment.Error, "dependency_failed:") {
				continue
			}
			if assignment.Status == AssignmentCancelled && cancelRequested && assignment.Error == "cancelled before worker dispatch" {
				continue
			}
			return fmt.Errorf("multi_agent: owned assignment %s has no durable terminal child", assignment.ID)
		}
		child, found, err := c.tracker.Get(ctx, assignment.ChildInteractionID)
		if err != nil {
			return fmt.Errorf("multi_agent: read terminal proof for %s: %w", assignment.ID, err)
		}
		if !found || child == nil {
			return fmt.Errorf("multi_agent: owned assignment %s has no durable child receipt", assignment.ID)
		}
		if err := validateReconciledChildScope(child, assignment, spaceID, conversationID); err != nil {
			return err
		}
		if err := validateTerminalChildSnapshot(child, CoordinationAssignmentSnapshot{
			AssignmentID:       assignment.ID,
			ChildInteractionID: assignment.ChildInteractionID,
			Status:             assignment.Status,
		}); err != nil {
			return fmt.Errorf("multi_agent: owned assignment %s lacks matching terminal proof: %w", assignment.ID, err)
		}
	}
	return nil
}
