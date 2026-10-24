package workbenchtasks

import "testing"

func TestRound59ReconciliationNeverCountsAsConfirmedSuccess(t *testing.T) {
	cases := []struct {
		status     string
		wantActive int
	}{
		{"succeeded", 1}, {"failed", 1}, {"cancelled", 1}, {"paused", 1}, {"running", 1},
	}
	for _, test := range cases {
		t.Run(test.status, func(t *testing.T) {
			tasks := []Task{{Status: test.status, RecoveryState: "needs-reconciliation", Assignments: []Assignment{{ID: "w1", ReviewRequired: true}}}}
			got := summarizeTasks(tasks)
			if got.Active != test.wantActive || got.Succeeded != 0 || got.Failed != 0 || got.Paused != 0 || got.NeedsReconciliation != 1 || got.Total != 1 || got.CodingWorkersRequiringReview != 1 {
				t.Fatalf("unverified state misrepresented: %+v", got)
			}
		})
	}
}

func TestRound59VerifiedTerminalStatusesStillCount(t *testing.T) {
	tasks := []Task{
		{Status: "succeeded", RecoveryState: "runtime-observed"},
		{Status: "failed", RecoveryState: "checkpoint-only"},
		{Status: "paused", RecoveryState: "checkpoint-only"},
		{Status: "running", RecoveryState: "runtime-observed"},
	}
	got := summarizeTasks(tasks)
	if got.Total != 4 || got.Active != 1 || got.Succeeded != 1 || got.Failed != 1 || got.Paused != 1 || got.NeedsReconciliation != 0 {
		t.Fatalf("verified states were changed: %+v", got)
	}
}
