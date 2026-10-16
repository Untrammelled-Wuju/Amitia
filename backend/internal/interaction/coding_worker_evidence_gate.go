package interaction

type CodingWorkerVerificationGate struct {
	State                string `json:"state"`
	Code                 string `json:"code"`
	EligibleForMerge     bool   `json:"eligibleForMerge"`
	ConfirmedMerged      bool   `json:"confirmedMerged"`
	ManualActionRequired bool   `json:"manualActionRequired"`
}

func EvaluateCodingWorkerVerification(status AgentAssignmentStatus, evidence *CodingWorktreeReview) CodingWorkerVerificationGate {
	gate := CodingWorkerVerificationGate{State: "blocked", Code: "WORKER_NOT_FINISHED"}
	switch status {
	case AssignmentFailed, AssignmentCancelled:
		gate.Code = "WORKER_FAILED_OR_CANCELLED"
		return gate
	case AssignmentSucceeded:
	default:
		gate.State = "waiting"
		return gate
	}
	if evidence == nil || evidence.State != "ready" || evidence.ReviewIncomplete || evidence.ErrorCode != "" || evidence.WorktreeID == "" || evidence.WorkspaceURI == "" || evidence.CurrentHead == "" || evidence.BaselineHead == "" || evidence.CurrentHead == evidence.BaselineHead || evidence.FilesChanged <= 0 || evidence.ConflictedCount > 0 || evidence.UntrackedCount > 0 {
		gate.Code = "REVIEW_INCOMPLETE_OR_UNAVAILABLE"
		gate.ManualActionRequired = true
		return gate
	}
	if !verifiedCodingTestProjection(evidence) {
		gate.Code = "VERIFIED_TEST_REQUIRED"
		if evidence.TestState == "failed" {
			gate.Code = "TEST_FAILED"
		}
		gate.ManualActionRequired = true
		return gate
	}
	if evidence.ApprovalState != "approved" {
		gate.Code = "HUMAN_APPROVAL_REQUIRED"
		gate.ManualActionRequired = true
		return gate
	}
	if evidence.Terminal != nil && evidence.Terminal.UnverifiedActive > 0 {
		gate.Code = "ACTIVE_TERMINAL_REQUIRES_REVIEW"
		gate.ManualActionRequired = true
		return gate
	}
	gate.EligibleForMerge = true
	switch evidence.MergeState {
	case "applied":
		if evidence.MergeAppliedHead == evidence.CurrentHead && evidence.MergeAppliedHead != "" {
			gate.State = "merged"
			gate.Code = "MERGE_CONFIRMED"
			gate.ConfirmedMerged = true
			gate.EligibleForMerge = false
			return gate
		}
		gate.State = "blocked"
		gate.Code = "MERGE_AUDIT_INCONSISTENT"
		gate.EligibleForMerge = false
	case "pending", "failed-needs-review", "unavailable":
		gate.State = "blocked"
		gate.Code = "MERGE_REQUIRES_RECONCILIATION"
		gate.EligibleForMerge = false
	case "not_attempted":
		gate.State = "approved"
		gate.Code = "READY_FOR_MANUAL_MERGE"
	default:
		gate.State = "blocked"
		gate.Code = "MERGE_AUDIT_INCONSISTENT"
		gate.EligibleForMerge = false
	}
	gate.ManualActionRequired = true
	return gate
}
