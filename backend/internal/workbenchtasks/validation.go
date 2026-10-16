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
	ids := make(map[string]struct{}, len(checkpoint.AssignmentRefs))
	for _, child := range checkpoint.AssignmentRefs {
		id := strings.TrimSpace(child.AssignmentID)
		if id == "" || len(id) > 160 || len(child.Role) > 100 || !validateAssignment(child.Status) || len([]rune(child.Objective)) > 16000 {
			return false
		}
		if _, existing := ids[id]; existing {
			return false
		}
		ids[id] = struct{}{}
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
