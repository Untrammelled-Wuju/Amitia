package interaction

type CodingWorkerEvidenceSummary struct {
	Total              int  `json:"total"`
	Waiting            int  `json:"waiting"`
	Blocked            int  `json:"blocked"`
	Approved           int  `json:"approved"`
	ConfirmedMerged    int  `json:"confirmedMerged"`
	RequiresAttention  bool `json:"requiresAttention"`
	AllMergesConfirmed bool `json:"allMergesConfirmed"`
}

func SummarizeCodingWorkerEvidence(assignments []CoordinationAssignmentSnapshot) *CodingWorkerEvidenceSummary {
	result := &CodingWorkerEvidenceSummary{}
	for i := range assignments {
		assignment := &assignments[i]
		if assignment.WorkerRole != "coding_worker" {
			continue
		}
		result.Total++
		gate := EvaluateCodingWorkerVerification(assignment.Status, assignment.WorktreeReview)
		switch gate.State {
		case "waiting":
			result.Waiting++
		case "merged":
			result.ConfirmedMerged++
		case "approved":
			result.Approved++
		default:
			result.Blocked++
		}
	}
	if result.Total == 0 {
		return nil
	}
	result.RequiresAttention = result.Blocked > 0 || result.Approved > 0
	result.AllMergesConfirmed = result.ConfirmedMerged == result.Total
	return result
}
