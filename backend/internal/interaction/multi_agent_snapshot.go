package interaction

import (
	"context"
	"fmt"
)

type CoordinationSnapshot struct {
	CoordinationID      string                           `json:"coordinationId"`
	ParentInteractionID string                           `json:"parentInteractionId"`
	Status              CoordinationStatus               `json:"status"`
	SourceWorkspaceURI  string                           `json:"-"`
	Assignments         []CoordinationAssignmentSnapshot `json:"assignments"`
	VerificationSummary *CodingWorkerEvidenceSummary     `json:"verificationSummary,omitempty"`
}

type CoordinationAssignmentSnapshot struct {
	AssignmentID       string                `json:"assignmentId"`
	ChildInteractionID string                `json:"childInteractionId,omitempty"`
	Status             AgentAssignmentStatus `json:"status"`
	WorkerRole         string                `json:"workerRole,omitempty"`
	WorktreeReview     *CodingWorktreeReview `json:"worktreeReview,omitempty"`
	Objective          string                `json:"objective"`
	ResultRef          string                `json:"resultRef,omitempty"`
	Error              string                `json:"error,omitempty"`
}

func (c *MultiAgentCoordinator) Snapshot(ctx context.Context, coordinationID string) (CoordinationSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return CoordinationSnapshot{}, err
	}
	if c == nil {
		return CoordinationSnapshot{}, fmt.Errorf("multi_agent: coordinator unavailable")
	}
	c.mu.Lock()
	ac, found := c.coordinations[coordinationID]
	if !found {
		c.mu.Unlock()
		return CoordinationSnapshot{}, fmt.Errorf("multi_agent: coordination not found")
	}
	out := CoordinationSnapshot{
		CoordinationID: coordinationID, ParentInteractionID: ac.parentInteractionID,
		Status: ac.status, SourceWorkspaceURI: ac.workspaceRootURI,
		Assignments: make([]CoordinationAssignmentSnapshot, 0, len(ac.assignments)),
	}
	identities := make(map[string]AgentAssignment, len(ac.assignments))
	for _, a := range ac.assignments {
		identities[a.ID] = AgentAssignment{ID: a.ID, ChildInteractionID: a.ChildInteractionID, WorkerRef: a.WorkerRef}
		out.Assignments = append(out.Assignments, CoordinationAssignmentSnapshot{
			AssignmentID: a.ID, ChildInteractionID: a.ChildInteractionID, Status: a.Status,
			WorkerRole: a.WorkerRef.Role, Objective: a.Objective, Error: a.Error,
		})
	}
	c.mu.Unlock()
	if c.tracker != nil {
		for i := range out.Assignments {
			assignment := &out.Assignments[i]
			if !assignment.Status.IsTerminal() || assignment.ChildInteractionID == "" {
				continue
			}
			child, found, err := c.tracker.Get(ctx, assignment.ChildInteractionID)
			if err != nil {
				return CoordinationSnapshot{}, fmt.Errorf("multi_agent: read worker result: %w", err)
			}
			if found && child != nil {
				if err := validateTerminalChildSnapshot(child, *assignment); err != nil {
					return CoordinationSnapshot{}, err
				}
				spaceID, conversationID, scopeErr := c.resolveWorkerScope(ctx, ac)
				if scopeErr != nil {
					return CoordinationSnapshot{}, scopeErr
				}
				if err := validateReconciledChildScope(child, identities[assignment.AssignmentID], spaceID, conversationID); err != nil {
					return CoordinationSnapshot{}, err
				}
				assignment.ResultRef = child.ResultRef
				if assignment.Error == "" && child.ErrorMessage != "" {
					assignment.Error = child.ErrorMessage
				}
			}
		}
	}
	return out, nil
}
