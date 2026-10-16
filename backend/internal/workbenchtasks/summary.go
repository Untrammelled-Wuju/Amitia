package workbenchtasks

func summarizeTasks(tasks []Task) TaskSummary {
	var out TaskSummary
	out.Total = len(tasks)
	for _, task := range tasks {
		switch task.RecoveryState {
		case "runtime-observed":
			out.RuntimeObserved++
		case "needs-reconciliation":
			out.NeedsReconciliation++
		default:
			out.CheckpointOnly++
		}
		switch task.Status {
		case "succeeded":
			out.Succeeded++
		case "failed", "cancelled":
			out.Failed++
		case "paused":
			out.Paused++
		default:
			out.Active++
		}
		for _, child := range task.Assignments {
			if child.ReviewRequired {
				out.CodingWorkersRequiringReview++
			}
		}
	}
	return out
}
