package interaction

import "testing"

func TestCodingWorkerEvidenceAggregateDoesNotTrustWorkerSuccess(t *testing.T) {
	base := &CodingWorktreeReview{State: "ready", WorktreeID: "0123456789abcdef0123456789abcdef", WorkspaceURI: "amitia://workspace/@task/", BaselineHead: "a", CurrentHead: "b", FilesChanged: 1, CheckpointRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", TestState: "passed", TestProfile: "go", TestOutputDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", TestStartedAt: "2026-10-14T12:00:00Z", ApprovalState: "approved", MergeState: "not_attempted"}
	all := []CoordinationAssignmentSnapshot{
		{WorkerRole: "coding_worker", Status: AssignmentSucceeded, WorktreeReview: base},
		{WorkerRole: "coding_worker", Status: AssignmentSucceeded},
		{WorkerRole: "coding_worker", Status: AssignmentRunning},
		{WorkerRole: "reviewer", Status: AssignmentSucceeded},
	}
	summary := SummarizeCodingWorkerEvidence(all)
	if summary == nil || summary.Total != 3 || summary.Waiting != 1 || summary.Blocked != 1 || summary.Approved != 1 || summary.ConfirmedMerged != 0 || summary.AllMergesConfirmed {
		t.Fatalf("invalid aggregate: %+v", summary)
	}
	base.MergeState = "applied"
	base.MergeAppliedHead = base.CurrentHead
	all = []CoordinationAssignmentSnapshot{{WorkerRole: "coding_worker", Status: AssignmentSucceeded, WorktreeReview: base}}
	summary = SummarizeCodingWorkerEvidence(all)
	if summary == nil || !summary.AllMergesConfirmed || summary.ConfirmedMerged != 1 || summary.RequiresAttention {
		t.Fatalf("verified merge not recognized: %+v", summary)
	}
	if SummarizeCodingWorkerEvidence([]CoordinationAssignmentSnapshot{{WorkerRole: "reviewer", Status: AssignmentSucceeded}}) != nil {
		t.Fatal("noncoding task polluted coding evidence")
	}
}
