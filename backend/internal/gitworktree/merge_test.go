package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func approvedMergeFixture(t *testing.T) (*Manager, string, Entry, string) {
	t.Helper()
	m, root, entry := checkpointFixture(t)
	if err := os.WriteFile(filepath.Join(entry.Path, "go.mod"), []byte("module sandbox.example/merge\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "merge_test.go"), []byte("package merge\nimport \"testing\"\nfunc TestReady(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "go.mod", "merge_test.go")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "agent change")
	ctx := context.Background()
	checkpoint, err := m.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "go")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", checkpoint.Revision, "local-owner")
	if err != nil {
		t.Fatal(err)
	}
	return m, root, entry, checkpoint.Revision
}

func TestApprovedMergePreviewAndApply(t *testing.T) {
	m, root, entry, _ := approvedMergeFixture(t)
	ctx := context.Background()
	preview, err := m.PreviewApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FilesChanged != 2 || preview.PlanID == "" || preview.ApprovedBy != "local-owner" {
		t.Fatalf("bad plan: %+v", preview)
	}
	if gitTest(t, root, "rev-parse", "HEAD") != preview.TargetHead {
		t.Fatal("preview modified target")
	}
	if _, err = m.ApplyApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1", preview.PlanID, "other-owner"); !errors.Is(err, ErrApprovalDenied) {
		t.Fatalf("wrong actor: %v", err)
	}
	if _, err = m.ApplyApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1", "wrong", "local-owner"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid token: %v", err)
	}
	merged, err := m.ApplyApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1", preview.PlanID, "local-owner")
	if err != nil || merged.Result != "applied" || gitTest(t, root, "rev-parse", "HEAD") != preview.SourceHead {
		t.Fatalf("merge: %+v %v", merged, err)
	}
	fresh, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := fresh.ReadMergeAudit(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Plan.PlanID != preview.PlanID || audit.Result != "applied" {
		t.Fatalf("audit recovery: %+v %v", audit, err)
	}
	if _, err = m.ApplyApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1", preview.PlanID, "local-owner"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("duplicate merge: %v", err)
	}
}

func TestApprovedMergeRejectsStaleTargetAndDirty(t *testing.T) {
	m, root, entry, _ := approvedMergeFixture(t)
	ctx := context.Background()
	plan, err := m.PreviewApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1", plan.PlanID, "local-owner"); !errors.Is(err, ErrDirty) {
		t.Fatalf("dirty target merge accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "staged.txt")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "target advanced")
	if _, err = m.ApplyApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1", plan.PlanID, "local-owner"); !errors.Is(err, ErrMergeNotFastForward) {
		t.Fatalf("diverged target merge accepted: %v", err)
	}
	if _, err = m.ReadMergeAudit(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrCheckpointRequired) {
		t.Fatalf("unsuccessful preflight wrote audit: %v", err)
	}
}

func TestApprovedMergeRejectsChangedBranchAndUnapproved(t *testing.T) {
	m, root, entry, _ := approvedMergeFixture(t)
	plan, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "checkout", "-b", "alternate")
	if _, err = m.ApplyApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1", plan.PlanID, "local-owner"); !errors.Is(err, ErrMergeTargetChanged) {
		t.Fatalf("branch switched but applied: %v", err)
	}
	other, _, work := checkpointFixture(t)
	if _, err = other.PreviewApprovedMerge(context.Background(), work.Path, work.ID, "task-1", "conversation-1"); err == nil {
		t.Fatal("source worktree used as merge target")
	}
}

func TestApprovedMergeAuditSymlinkRejected(t *testing.T) {
	m, root, entry, _ := approvedMergeFixture(t)
	_, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(m.mergeAuditPath(key, entry.ID))
	if err = os.Symlink(t.TempDir(), dir); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err = m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("audit symlink accepted: %v", err)
	}
}

func TestApprovedMergeConcurrentApplyOnlyOnce(t *testing.T) {
	m, root, entry, _ := approvedMergeFixture(t)
	plan, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	successes := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.ApplyApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1", plan.PlanID, "local-owner")
			successes <- err == nil
		}()
	}
	wg.Wait()
	close(successes)
	total := 0
	for ok := range successes {
		if ok {
			total++
		}
	}
	if total != 1 {
		t.Fatalf("expected one merge, got %d", total)
	}
}

func TestApprovedMergeRequiresApprovalAndNoEarlierAttempt(t *testing.T) {
	m, root, entry := checkpointFixture(t)
	if _, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrApprovalDenied) {
		t.Fatalf("unapproved merge preview accepted: %v", err)
	}
}

func TestApprovedMergeBlocksPendingAudit(t *testing.T) {
	m, root, entry, _ := approvedMergeFixture(t)
	plan, err := m.PreviewApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.writeMergeAudit(key, entry.ID, MergeAudit{Version: 1, Plan: *plan, Actor: "local-owner", StartedAt: time.Now().UTC(), Result: "pending"}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyApprovedMerge(context.Background(), root, entry.ID, "task-1", "conversation-1", plan.PlanID, "local-owner"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("pending audit retry accepted: %v", err)
	}
	if gitTest(t, root, "rev-parse", "HEAD") != plan.TargetHead {
		t.Fatal("pending attempt modified target")
	}
}
