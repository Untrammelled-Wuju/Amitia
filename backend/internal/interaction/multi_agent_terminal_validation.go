package interaction

import "fmt"

func validateTerminalAssignmentResult(target *AgentAssignment, assignmentID string, result AgentAssignmentResult) error {
	if target == nil || target.ID == "" || assignmentID == "" || target.ID != assignmentID ||
		result.AssignmentID != assignmentID || !result.Status.IsTerminal() {
		return fmt.Errorf("multi_agent: terminal result identity or status mismatch")
	}
	if !target.Status.IsTerminal() && (target.Status == AssignmentPending || target.ChildInteractionID == "") {
		return fmt.Errorf("multi_agent: terminal result for worker without durable dispatch")
	}
	if target.ChildInteractionID != result.ChildInteractionID {
		return fmt.Errorf("multi_agent: terminal result does not match durable child interaction")
	}
	if target.Status.IsTerminal() && target.Status != result.Status {
		return fmt.Errorf("multi_agent: conflicting terminal result for completed assignment")
	}
	return nil
}
