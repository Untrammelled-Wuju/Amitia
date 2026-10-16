package interaction

import "testing"

func TestCodingWorkerVerificationRequiresIndependentEvidence(t *testing.T) {
	valid := func() *CodingWorktreeReview {
		return &CodingWorktreeReview{
			State: "ready", WorktreeID: "0123456789abcdef0123456789abcdef", WorkspaceURI: "amitia://workspace/@task/",
			BaselineHead: "0000000000000000000000000000000000000000", CurrentHead: "1111111111111111111111111111111111111111",
			FilesChanged: 1, CheckpointRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", TestState: "passed", TestProfile: "go", TestOutputDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", TestStartedAt: "2026-10-14T12:00:00Z", ApprovalState: "approved", MergeState: "not_attempted",
		}
	}
	for _, tc := range []struct {
		name     string
		status   AgentAssignmentStatus
		mutate   func(*CodingWorktreeReview)
		code     string
		eligible bool
	}{
		{"running", AssignmentRunning, func(*CodingWorktreeReview) {}, "WORKER_NOT_FINISHED", false},
		{"failed", AssignmentFailed, func(*CodingWorktreeReview) {}, "WORKER_FAILED_OR_CANCELLED", false},
		{"incomplete", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.UntrackedCount = 1 }, "REVIEW_INCOMPLETE_OR_UNAVAILABLE", false},
		{"empty-diff", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.FilesChanged = 0 }, "REVIEW_INCOMPLETE_OR_UNAVAILABLE", false},
		{"same-head", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.CurrentHead = e.BaselineHead }, "REVIEW_INCOMPLETE_OR_UNAVAILABLE", false},
		{"no-checkpoint", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.CheckpointRevision = "" }, "VERIFIED_TEST_REQUIRED", false},
		{"no-test", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.TestState = "not_run" }, "VERIFIED_TEST_REQUIRED", false},
		{"failed-test", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.TestState = "failed" }, "TEST_FAILED", false},
		{"exit-failed", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.TestExitCode = 1 }, "VERIFIED_TEST_REQUIRED", false},
		{"missing-digest", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.TestOutputDigest = "" }, "VERIFIED_TEST_REQUIRED", false},
		{"invalid-profile", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.TestProfile = "unknown" }, "VERIFIED_TEST_REQUIRED", false},
		{"bad-started-at", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.TestStartedAt = "n/a" }, "VERIFIED_TEST_REQUIRED", false},
		{"no-approval", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.ApprovalState = "pending" }, "HUMAN_APPROVAL_REQUIRED", false},
		{"approved", AssignmentSucceeded, func(*CodingWorktreeReview) {}, "READY_FOR_MANUAL_MERGE", true},
		{"running-terminal", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.Terminal = &CodingTerminalDiagnostics{UnverifiedActive: 1} }, "ACTIVE_TERMINAL_REQUIRES_REVIEW", false},
		{"unknown-audit", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.MergeState = "unexpected" }, "MERGE_AUDIT_INCONSISTENT", false},
		{"empty-audit", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.MergeState = "" }, "MERGE_AUDIT_INCONSISTENT", false},
		{"merge-pending", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.MergeState = "pending" }, "MERGE_REQUIRES_RECONCILIATION", false},
		{"audit-mismatch", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.MergeState = "applied"; e.MergeAppliedHead = "mismatch" }, "MERGE_AUDIT_INCONSISTENT", false},
		{"merged", AssignmentSucceeded, func(e *CodingWorktreeReview) { e.MergeState = "applied"; e.MergeAppliedHead = e.CurrentHead }, "MERGE_CONFIRMED", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			tc.mutate(r)
			gate := EvaluateCodingWorkerVerification(tc.status, r)
			if gate.Code != tc.code || gate.EligibleForMerge != tc.eligible {
				t.Fatalf("unexpected verification gate: %+v", gate)
			}
			if tc.code == "MERGE_CONFIRMED" && !gate.ConfirmedMerged {
				t.Fatal("merged state not confirmed")
			}
		})
	}
	gate := EvaluateCodingWorkerVerification(AssignmentSucceeded, nil)
	if gate.Code != "REVIEW_INCOMPLETE_OR_UNAVAILABLE" {
		t.Fatal("missing evidence accepted")
	}
}

func TestTerminalExitCodeNeverGrantsCodingApproval(t *testing.T) {
	zero := 0
	review := &CodingWorktreeReview{State: "ready", WorktreeID: "0123456789abcdef0123456789abcdef", WorkspaceURI: "amitia://workspace/@task/", BaselineHead: "abc", CurrentHead: "def", FilesChanged: 1, Terminal: &CodingTerminalDiagnostics{State: "recorded", Total: 1, Exited: 1, LastExitCode: &zero}}
	result := EvaluateCodingWorkerVerification(AssignmentSucceeded, review)
	if result.EligibleForMerge || result.ConfirmedMerged || result.Code != "VERIFIED_TEST_REQUIRED" {
		t.Fatalf("terminal exit mistakenly treated as test: %+v", result)
	}
}
