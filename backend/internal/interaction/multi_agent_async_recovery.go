package interaction

import (
	"context"
	"fmt"
)

func (c *MultiAgentCoordinator) resumeReservedWorker(ctx context.Context, ac *activeCoordination, assignment AgentAssignment) error {
	return c.withOwnedExecution(ctx, ac, func(ctx context.Context) error { return c.resumeAuthorizedWorker(ctx, ac, assignment) })
}

func (c *MultiAgentCoordinator) resumeAuthorizedWorker(ctx context.Context, ac *activeCoordination, assignment AgentAssignment) error {
	if err := c.requireCurrentCoordinationSnapshot(ctx, ac); err != nil {
		return err
	}
	if err := c.requireDispatchableParent(ctx, ac); err != nil {
		return err
	}
	reserved, ok := c.starter.(interface{ WorkerInteractionID(WorkerRunRequest) string })
	if !ok || assignment.Status != AssignmentRunning || assignment.ChildInteractionID == "" {
		return nil
	}
	if ac.status != CoordinationRunning {
		return nil
	}
	spaceID, conversationID, err := c.resolveWorkerScope(ctx, ac)
	if err != nil {
		return err
	}
	request := WorkerRunRequest{
		OwnedScope:     ac.ownedScope,
		CoordinationID: string(ac.id), AssignmentID: assignment.ID, WorkerRole: assignment.WorkerRef.Role,
		ParentInteractionID: ac.parentInteractionID, ParentGoalID: ac.parentGoalID,
		ParentGoalRevision: ac.parentGoalRev, CharacterID: assignment.WorkerRef.CharacterID,
		ConversationID: conversationID, SpaceID: spaceID, WorkspaceID: ac.workspaceID,
		PermissionMode: ac.permissionMode, WorkspaceDeviceID: ac.workspaceDeviceID,
		WorkspaceName: ac.workspaceName, WorkspaceKind: ac.workspaceKind,
		WorkspaceRootURI: ac.workspaceRootURI, Source: "multi_agent",
		Objective: assignment.Objective, ExpectedOutcome: assignment.ExpectedOutcome,
		CoordinationDepth: ac.depth + 1,
	}
	if expected := reserved.WorkerInteractionID(request); expected != assignment.ChildInteractionID {
		return fmt.Errorf("multi_agent: recovered worker %s has mismatched durable child ID", assignment.ID)
	}
	if c.tracker == nil {
		return fmt.Errorf("multi_agent: tracker unavailable to verify reserved child %s", assignment.ID)
	}
	child, found, err := c.tracker.Get(ctx, assignment.ChildInteractionID)
	if err != nil {
		return err
	}
	if !found || child == nil || child.Status != InteractionStatusReceived {
		return fmt.Errorf("multi_agent: worker %s has no unstarted durable child reservation; manual reconciliation required", assignment.ID)
	}
	if err := validateReconciledChildScope(child, assignment, spaceID, conversationID); err != nil {
		return err
	}
	childID, err := c.starter.StartWorker(ctx, request)
	if err != nil {
		return fmt.Errorf("multi_agent: resume reserved worker %s: %w", assignment.ID, err)
	}
	if childID != assignment.ChildInteractionID {
		return fmt.Errorf("multi_agent: resumed worker %s changed its interaction ID", assignment.ID)
	}
	return nil
}

func (c *MultiAgentCoordinator) Shutdown(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if stopper, ok := c.starter.(interface{ Shutdown(context.Context) error }); ok {
		return stopper.Shutdown(ctx)
	}
	return nil
}

func (c *MultiAgentCoordinator) waitForMissingChildProof(ctx context.Context, ac *activeCoordination, assignment AgentAssignment) error {
	const reason = "durable child reservation missing; previous dispatch outcome is unknown and requires manual reconciliation"
	c.mu.Lock()
	changed := false
	for _, live := range ac.assignments {
		if live.ID != assignment.ID || live.Status.IsTerminal() {
			continue
		}
		if live.Status != AssignmentWaiting || live.Error != reason || ac.status != CoordinationWaiting {
			live.Status = AssignmentWaiting
			live.Error = reason
			live.UpdatedAt = c.clock.Now()
			ac.status = CoordinationWaiting
			ac.updatedAt = live.UpdatedAt
			changed = true
		}
		break
	}
	c.mu.Unlock()
	if changed {
		return c.persistCoordinationSnapshot(ctx, ac)
	}
	return nil
}
