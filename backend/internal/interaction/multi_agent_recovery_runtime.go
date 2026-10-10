package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

func (c *MultiAgentCoordinator) persistCoordinationSnapshot(ctx context.Context, ac *activeCoordination) error {
	if c == nil || ac == nil || ac.parentInteractionID == "" {
		return nil
	}
	ac.persistMu.Lock()
	defer ac.persistMu.Unlock()
	c.mu.Lock()
	ref := &MultiAgentRecoveryRef{
		CoordinationID: string(ac.id), ParentGoalID: ac.parentGoalID,
		ParentGoalRevision: ac.parentGoalRev, Status: string(ac.status),
		Strategy: ac.strategy, CompletionPlan: ac.completionPlan,
		Depth: ac.depth, DeadlineAt: ac.deadlineAt,
		WorkspaceID: ac.workspaceID, PermissionMode: ac.permissionMode,
		WorkspaceDeviceID: ac.workspaceDeviceID,
		WorkspaceName:     ac.workspaceName, WorkspaceKind: ac.workspaceKind,
		WorkspaceRootURI: ac.workspaceRootURI,
		AssignmentRefs:   make([]AssignmentRecoveryRef, 0, len(ac.assignments)),
	}
	for _, assignment := range ac.assignments {
		ref.AssignmentRefs = append(ref.AssignmentRefs, AssignmentRecoveryRef{
			AssignmentID: assignment.ID, WorkerID: assignment.WorkerRef.WorkerID,
			CharacterID: assignment.WorkerRef.CharacterID, Role: assignment.WorkerRef.Role,
			Objective: assignment.Objective, ExpectedOutcome: assignment.ExpectedOutcome,
			Constraints:        append([]string(nil), assignment.Constraints...),
			Dependencies:       append([]string(nil), assignment.Dependencies...),
			ChildInteractionID: assignment.ChildInteractionID,
			ChildGoalID:        assignment.ChildGoalID, Status: string(assignment.Status),
			Error: assignment.Error,
		})
	}
	parentID := ac.parentInteractionID
	c.mu.Unlock()
	if c.goals != nil {
		if goal, found := c.goals.Get(ac.parentGoalID); found {
			ref.ParentGoalSpaceID = goal.SpaceID
			ref.ParentGoalCharacterID = goal.CharacterID
			ref.ParentGoalConversationID = goal.ConversationID
			ref.ParentGoalDescription = goal.Description
		}
	}
	return c.attachRecoveryDescriptor(ctx, parentID, ref)
}

func (c *MultiAgentCoordinator) restoreCoordinationFromRecord(record *InteractionRecord) error {
	if c == nil || record == nil || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil {
		return nil
	}
	ref := record.RecoveryDescriptor.MultiAgent
	if record.RecoveryDescriptor.Fingerprint != "" {
		copyOfDescriptor := *record.RecoveryDescriptor
		copyOfDescriptor.ComputeFingerprint()
		if copyOfDescriptor.Fingerprint != record.RecoveryDescriptor.Fingerprint {
			return fmt.Errorf("multi_agent: recovered coordination descriptor fingerprint mismatch")
		}
	}
	if ref.CoordinationID == "" {
		return fmt.Errorf("multi_agent: recovery descriptor has no coordination ID")
	}
	switch CoordinationStatus(ref.Status) {
	case CoordinationPlanning, CoordinationRunning, CoordinationWaiting, CoordinationAggregating,
		CoordinationSucceeded, CoordinationFailed, CoordinationCancelled, CoordinationPaused:
	default:
		return fmt.Errorf("multi_agent: recovered coordination has unknown state %q", ref.Status)
	}
	c.mu.Lock()
	if _, exists := c.coordinations[ref.CoordinationID]; exists {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if ref.Strategy == "" || ref.CompletionPlan == "" || len(ref.AssignmentRefs) == 0 {
		return fmt.Errorf("multi_agent: coordination %s lacks durable scheduling details; manual reconciliation is required", ref.CoordinationID)
	}
	if c.goals == nil {
		return fmt.Errorf("multi_agent: goal registry unavailable during recovery")
	}
	if _, found := c.goals.Get(ref.ParentGoalID); !found {
		scope := record.Scope.Normalize()
		if ref.ParentGoalSpaceID == "" || ref.ParentGoalConversationID == "" ||
			ref.ParentGoalSpaceID != scope.SpaceID || ref.ParentGoalConversationID != scope.ConversationID {
			return fmt.Errorf("multi_agent: durable parent goal scope is missing or mismatched")
		}
		if err := c.goals.Register(decision.Goal{
			ID: ref.ParentGoalID, Revision: ref.ParentGoalRevision,
			SpaceID: ref.ParentGoalSpaceID, CharacterID: ref.ParentGoalCharacterID,
			ConversationID: ref.ParentGoalConversationID,
			Description:    ref.ParentGoalDescription, Status: decision.GoalStatusActive,
		}); err != nil {
			return fmt.Errorf("multi_agent: restore parent goal: %w", err)
		}
	}
	now := time.Now().UTC()
	ac := &activeCoordination{
		id:           CoordinationID(ref.CoordinationID),
		parentGoalID: ref.ParentGoalID, parentGoalRev: ref.ParentGoalRevision,
		parentInteractionID: record.ID, status: CoordinationStatus(ref.Status),
		workspaceID: ref.WorkspaceID, permissionMode: ref.PermissionMode,
		workspaceDeviceID: ref.WorkspaceDeviceID,
		workspaceName:     ref.WorkspaceName, workspaceKind: ref.WorkspaceKind,
		workspaceRootURI: ref.WorkspaceRootURI,
		strategy:         ref.Strategy, completionPlan: ref.CompletionPlan,
		depth: ref.Depth, deadlineAt: ref.DeadlineAt,
		assignments: make([]*AgentAssignment, 0, len(ref.AssignmentRefs)),
		createdAt:   now, updatedAt: now,
	}
	for _, saved := range ref.AssignmentRefs {
		if saved.AssignmentID == "" || saved.WorkerID == "" || saved.Objective == "" {
			return fmt.Errorf("multi_agent: coordination %s has an incomplete worker checkpoint", ref.CoordinationID)
		}
		status := AgentAssignmentStatus(saved.Status)
		switch status {
		case AssignmentPending, AssignmentRunning, AssignmentPaused,
			AssignmentWaiting, AssignmentSucceeded, AssignmentFailed, AssignmentCancelled:
		default:
			return fmt.Errorf("multi_agent: unknown recovered assignment state %q", saved.Status)
		}
		if (status == AssignmentRunning || status == AssignmentPaused) && saved.ChildInteractionID == "" {
			return fmt.Errorf("multi_agent: worker %s was dispatched without a durable child interaction ID; prevent duplicate side effects", saved.AssignmentID)
		}
		ac.assignments = append(ac.assignments, &AgentAssignment{
			ID: saved.AssignmentID, CoordinationID: ref.CoordinationID,
			ParentGoalID: ref.ParentGoalID, ParentGoalRevision: ref.ParentGoalRevision,
			WorkerRef: AgentWorkerRef{WorkerID: saved.WorkerID, CharacterID: saved.CharacterID, Role: saved.Role},
			Objective: saved.Objective, ExpectedOutcome: saved.ExpectedOutcome,
			Constraints:  append([]string(nil), saved.Constraints...),
			Dependencies: append([]string(nil), saved.Dependencies...),
			Status:       status, ChildInteractionID: saved.ChildInteractionID,
			ChildGoalID: saved.ChildGoalID, Error: saved.Error,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	c.mu.Lock()
	if _, exists := c.coordinations[ref.CoordinationID]; !exists {
		c.coordinations[ref.CoordinationID] = ac
	}
	c.mu.Unlock()
	return nil
}

func (c *MultiAgentCoordinator) RecoverCoordination(ctx context.Context, record *InteractionRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.restoreCoordinationFromRecord(record)
}
