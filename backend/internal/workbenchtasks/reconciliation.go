package workbenchtasks

import "github.com/u-ai/backend/internal/interaction"

func reconciliationReason(record *interaction.InteractionRecord, checkpoint *interaction.MultiAgentRecoveryRef) string {
	if checkpoint == nil {
		return "parent-state-conflict"
	}
	checkpointState := checkpoint.Status
	if checkpointState == "succeeded" || checkpointState == "failed" || checkpointState == "cancelled" {
		for _, assignment := range checkpoint.AssignmentRefs {
			if assignment.Status != "succeeded" && assignment.Status != "failed" && assignment.Status != "cancelled" {
				return "parent-terminal-child-pending"
			}
		}
	}

	if validateTerminalAlignment(record, checkpointState) {
		return ""
	}
	switch checkpointState {
	case "succeeded", "failed", "cancelled":
		return ""
	default:
		return "parent-state-conflict"
	}
}

func markReconciliation(task *Task, reason string) {
	if task == nil || reason == "" {
		return
	}
	task.RecoveryState = "needs-reconciliation"
	if task.RecoveryReason == "" {
		task.RecoveryReason = reason
	}
}
