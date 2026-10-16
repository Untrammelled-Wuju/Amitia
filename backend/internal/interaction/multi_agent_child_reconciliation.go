package interaction

import "fmt"

func validateReconciledChildScope(record *InteractionRecord, assignment AgentAssignment, expectedSpaceID, expectedConversationID string) error {
	if record == nil || record.ID != assignment.ChildInteractionID {
		return fmt.Errorf("multi_agent: child interaction identity changed while reconciling")
	}
	if expectedSpaceID == "" && expectedConversationID == "" {
		return nil
	}
	scope := record.Scope.Normalize()
	if scope.SpaceID != expectedSpaceID || scope.ConversationID != workerConversationID(WorkerRunRequest{AssignmentID: assignment.ID}) ||
		scope.CharacterID != assignment.WorkerRef.CharacterID || scope.RequestID != "ma-"+assignment.ID {
		return fmt.Errorf("multi_agent: child interaction scope differs from durable assignment")
	}
	return nil
}
