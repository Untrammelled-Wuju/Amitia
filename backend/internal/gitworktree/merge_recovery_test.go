package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pendingMergeFixture(t *testing.T) (*Manager, string, Entry, *MergeProposal) {
	t.Helper()
	m, root, entry, _ := approvedMergeFixture(t)
	plan, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.writeMergeAudit(key, entry.ID, MergeAudit{Version: 1, Plan: *plan, Actor: plan.ApprovedBy, StartedAt: time.Now().UTC(), Result: "pending"}); err != nil {
		t.Fatal(err)
	}
	return m, root, entry, plan
}

func TestMergeRecoveryAfterCrashBetweenGitAndJournal(t *testing.T) {
	m, root, entry, plan := pendingMergeFixture(t)
	gitTest(t, root, "merge", "--ff-only", plan.SourceHead)
	restarted, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := restarted.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "applied" || audit.AppliedHead != plan.SourceHead || audit.RecoveredAt == nil || audit.FinishedAt == nil {
		t.Fatalf("merge recovery failed: %+v %v", audit, err)
	}
	again, err := m.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || again.RecoveredAt == nil || !again.RecoveredAt.Equal(*audit.RecoveredAt) {
		t.Fatalf("recovery not durable/idempotent: %+v %v", again, err)
	}
	if _, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1"); err != ErrMergePreviouslyAttempted {
		t.Fatalf("recovered merge can be replayed: %v", err)
	}
}

func TestMergeRecoveryDoesNotExecutePendingMerge(t *testing.T) {
	m, root, entry, plan := pendingMergeFixture(t)
	audit, err := m.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "pending" || audit.FinishedAt != nil {
		t.Fatalf("unapplied merge falsely recovered: %+v %v", audit, err)
	}
	if head := gitTest(t, root, "rev-parse", "HEAD"); head != plan.TargetHead {
		t.Fatalf("read unexpectedly merged repository: %s", head)
	}
}

func TestMergeRecoveryRejectsChangedTargetBranch(t *testing.T) {
	m, root, entry, plan := pendingMergeFixture(t)
	gitTest(t, root, "merge", "--ff-only", plan.SourceHead)
	gitTest(t, root, "checkout", "-b", "alternate-recovery")
	audit, err := m.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "pending" || audit.RecoveredAt != nil {
		t.Fatalf("wrong target branch accepted: %+v %v", audit, err)
	}
}

func TestMergeRecoveryRejectsDirtyTarget(t *testing.T) {
	m, root, entry, plan := pendingMergeFixture(t)
	gitTest(t, root, "merge", "--ff-only", plan.SourceHead)
	if err := os.WriteFile(filepath.Join(root, "untracked-merge.txt"), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	audit, err := m.ReadMergeAudit(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "pending" {
		t.Fatalf("dirty target accepted: %+v %v", audit, err)
	}
}
