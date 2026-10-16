package workbenchtasks

import "github.com/u-ai/backend/internal/interaction"

func reconciliationReason(record *interaction.InteractionRecord, checkpointState string) string {
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
