package workbenchterminal

import "testing"

func TestTaskReceiptSummaryStateAndEvidenceKind(t *testing.T) {
	code := 0
	rows := []TaskReceipt{
		{SessionID: "a", State: "exited", ExitCode: &code},
		{SessionID: "b", State: "interrupted"},
		{SessionID: "c", State: "revoked"},
	}
	result := SummarizeTaskReceipts(rows)
	if result.Total != 3 || result.Exited != 1 || result.Interrupted != 1 || result.Revoked != 1 || result.LatestSessionID != "a" || result.LastExitCode == nil || *result.LastExitCode != 0 || result.VerificationKind != "lifecycle-only-not-test-proof" {
		t.Fatalf("unexpected summary %+v", result)
	}
	empty := SummarizeTaskReceipts(nil)
	if empty.Total != 0 || empty.LatestState != "none" || empty.LastExitCode != nil {
		t.Fatalf("unexpected empty summary %+v", empty)
	}
}
