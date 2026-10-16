package interaction

import "fmt"

func validateTerminalChildSnapshot(child *InteractionRecord, assignment CoordinationAssignmentSnapshot) error {
	if child == nil || child.ID != assignment.ChildInteractionID || !assignment.Status.IsTerminal() || !child.IsTerminal() {
		return fmt.Errorf("multi_agent: terminal child snapshot is incomplete or no longer terminal")
	}
	switch assignment.Status {
	case AssignmentSucceeded:
		if child.Status != InteractionStatusCompleted {
			return fmt.Errorf("multi_agent: completed worker result conflicts with child state")
		}
	case AssignmentCancelled:
		if child.Status != InteractionStatusCancelled && child.Status != InteractionStatusSuperseded {
			return fmt.Errorf("multi_agent: cancelled worker result conflicts with child state")
		}
	case AssignmentFailed:
		if child.Status != InteractionStatusFailed && child.Status != InteractionStatusInterrupted {
			return fmt.Errorf("multi_agent: failed worker result conflicts with child state")
		}
	}
	return nil
}
