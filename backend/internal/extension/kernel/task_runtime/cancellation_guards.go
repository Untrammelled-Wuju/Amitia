package task_runtime

func remoteCancellationAllowed(status TaskRunStatus) bool {
	switch status {
	case RunStatusRunning, RunStatusCheckpointing, RunStatusPausing, RunStatusCancelling:
		return true
	default:
		return false
	}
}

func cancellationConfirmedForAttempt(before, after *TaskRun) bool {
	return before != nil && after != nil &&
		before.TaskRunID != "" && before.TaskRunID == after.TaskRunID &&
		before.Generation == after.Generation &&
		before.ExecutionAttemptID == after.ExecutionAttemptID &&
		after.Status == RunStatusCancelled
}
