package interaction

import (
	"fmt"

	"github.com/u-ai/backend/internal/decision"
)

func validateRecoveredParent(record *InteractionRecord, ref *MultiAgentRecoveryRef, goal decision.Goal) error {
	if record == nil || ref == nil || ref.ParentGoalID == "" || record.ID == "" ||
		goal.ID != ref.ParentGoalID || goal.Revision != ref.ParentGoalRevision {
		return fmt.Errorf("multi_agent: recovered parent goal identity or revision mismatch")
	}
	scope := record.Scope.Normalize()
	if scope.SpaceID != ref.ParentGoalSpaceID || scope.ConversationID != ref.ParentGoalConversationID ||
		goal.SpaceID != ref.ParentGoalSpaceID || goal.ConversationID != ref.ParentGoalConversationID ||
		goal.CharacterID != ref.ParentGoalCharacterID {
		return fmt.Errorf("multi_agent: recovered parent goal scope does not match parent interaction")
	}
	return nil
}

func validateRecoveredWorkerPlan(ref *MultiAgentRecoveryRef, maxAssignments, maxDepth int) error {
	if ref == nil || len(ref.AssignmentRefs) == 0 || len(ref.AssignmentRefs) > maxAssignments ||
		ref.Depth < 0 || ref.Depth > maxDepth {
		return fmt.Errorf("multi_agent: recovered worker plan exceeds scheduling bounds")
	}
	seen := make(map[string]AssignmentRecoveryRef, len(ref.AssignmentRefs))
	for _, assignment := range ref.AssignmentRefs {
		if assignment.AssignmentID == "" || assignment.WorkerID == "" || assignment.Objective == "" {
			return fmt.Errorf("multi_agent: incomplete recovered worker assignment")
		}
		if _, duplicate := seen[assignment.AssignmentID]; duplicate {
			return fmt.Errorf("multi_agent: duplicate recovered assignment %s", assignment.AssignmentID)
		}
		seen[assignment.AssignmentID] = assignment
	}
	visited := make(map[string]uint8, len(seen))
	var visit func(string) error
	visit = func(id string) error {
		if visited[id] == 1 {
			return fmt.Errorf("multi_agent: cyclic recovered worker dependency at %s", id)
		}
		if visited[id] == 2 {
			return nil
		}
		visited[id] = 1
		item := seen[id]
		deps := make(map[string]struct{}, len(item.Dependencies))
		for _, dependency := range item.Dependencies {
			if _, exists := seen[dependency]; !exists {
				return fmt.Errorf("multi_agent: unknown recovered worker dependency %s", dependency)
			}
			if _, duplicate := deps[dependency]; duplicate {
				return fmt.Errorf("multi_agent: duplicate recovered worker dependency %s", dependency)
			}
			deps[dependency] = struct{}{}
			if err := visit(dependency); err != nil {
				return err
			}
		}
		visited[id] = 2
		return nil
	}
	for id := range seen {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
