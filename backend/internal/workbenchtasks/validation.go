package workbenchtasks

import (
	"strings"

	"github.com/u-ai/backend/internal/interaction"
)

func validateCheckpoint(checkpoint *interaction.MultiAgentRecoveryRef) bool {
	if checkpoint == nil || strings.TrimSpace(checkpoint.CoordinationID) == "" || !validateState(checkpoint.Status) || len(checkpoint.AssignmentRefs) == 0 || len(checkpoint.AssignmentRefs) > 8 {
		return false
	}
	if len(checkpoint.CoordinationID) > 160 || len(checkpoint.ParentGoalSpaceID) > 160 || len(checkpoint.ParentGoalConversationID) > 160 {
		return false
	}
	if checkpoint.OwnedScope != nil {
		scope := checkpoint.OwnedScope
		if (checkpoint.ExecutionCoreID != "" && scope.CoreID != "" && checkpoint.ExecutionCoreID != scope.CoreID) ||
			(checkpoint.ResourceOwnerID != "" && scope.ResourceOwnerID != "" && checkpoint.ResourceOwnerID != scope.ResourceOwnerID) ||
			(checkpoint.ParentGoalSpaceID != "" && checkpoint.ParentGoalSpaceID != scope.SpaceID) ||
			(checkpoint.ParentGoalCharacterID != "" && checkpoint.ParentGoalCharacterID != scope.RoleID) {
			return false
		}
	}
	ids := make(map[string]struct{}, len(checkpoint.AssignmentRefs))
	childIDs := make(map[string]struct{}, len(checkpoint.AssignmentRefs))
	for _, child := range checkpoint.AssignmentRefs {
		id := strings.TrimSpace(child.AssignmentID)
		if id == "" || id != child.AssignmentID || len(id) > 160 || len(child.Dependencies) > 8 || len(child.Role) > 100 || !validateAssignment(child.Status) || len([]rune(child.Objective)) > 16000 {
			return false
		}
		if _, existing := ids[id]; existing {
			return false
		}
		ids[id] = struct{}{}
		if child.ChildInteractionID != "" {
			if _, reused := childIDs[child.ChildInteractionID]; reused {
				return false
			}
			childIDs[child.ChildInteractionID] = struct{}{}
		}
	}
	states := make(map[string]uint8, len(checkpoint.AssignmentRefs))
	byID := make(map[string]interaction.AssignmentRecoveryRef, len(checkpoint.AssignmentRefs))
	for _, child := range checkpoint.AssignmentRefs {
		byID[child.AssignmentID] = child
	}
	var visit func(string) bool
	visit = func(id string) bool {
		if states[id] == 1 {
			return false
		}
		if states[id] == 2 {
			return true
		}
		states[id] = 1
		dependencies := make(map[string]struct{})
		for _, dependency := range byID[id].Dependencies {
			if _, ok := byID[dependency]; !ok {
				return false
			}
			if _, duplicate := dependencies[dependency]; duplicate {
				return false
			}
			dependencies[dependency] = struct{}{}
			if !visit(dependency) {
				return false
			}
		}
		states[id] = 2
		return true
	}
	for id := range byID {
		if !visit(id) {
			return false
		}
	}
	return true
}

func validSnapshotTransition(checkpointState, runtimeState string, recordTerminal bool) bool {
	if checkpointState == "succeeded" || checkpointState == "failed" || checkpointState == "cancelled" {
		return runtimeState == checkpointState
	}
	if recordTerminal {
		return runtimeState == "succeeded" || runtimeState == "failed" || runtimeState == "cancelled"
	}
	return true
}
