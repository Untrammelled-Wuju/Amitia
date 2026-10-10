package interaction

import (
	"context"
	"fmt"
)

type CoordinationSnapshot struct {
	CoordinationID      string                           `json:"coordinationId"`
	ParentInteractionID string                           `json:"parentInteractionId"`
	Status              CoordinationStatus               `json:"status"`
	Assignments         []CoordinationAssignmentSnapshot `json:"assignments"`
}

type CoordinationAssignmentSnapshot struct {
	AssignmentID       string                `json:"assignmentId"`
	ChildInteractionID string                `json:"childInteractionId,omitempty"`
	Status             AgentAssignmentStatus `json:"status"`
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
		Status: ac.status, Assignments: make([]CoordinationAssignmentSnapshot, 0, len(ac.assignments)),
	}
	for _, a := range ac.assignments {
		out.Assignments = append(out.Assignments, CoordinationAssignmentSnapshot{
			AssignmentID: a.ID, ChildInteractionID: a.ChildInteractionID, Status: a.Status,
			Objective: a.Objective, Error: a.Error,
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
				assignment.ResultRef = child.ResultRef
				if assignment.Error == "" && child.ErrorMessage != "" {
					assignment.Error = child.ErrorMessage
				}
			}
		}
	}
	return out, nil
}
