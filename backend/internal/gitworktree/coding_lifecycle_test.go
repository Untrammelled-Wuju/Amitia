package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCodingTaskIsolatedLifecycleRecoveryApprovalAndMerge(t *testing.T) {
	ctx := context.Background()
	m, root := fixture(t)
	first, created, err := m.EnsureTask(ctx, root, "", "HEAD", "coding-a", "conversation-a")
	if err != nil || !created {
		t.Fatalf("first allocation: %+v %v", first, err)
	}
	second, created, err := m.EnsureTask(ctx, root, "", "HEAD", "coding-b", "conversation-a")
	if err != nil || !created || first.Path == second.Path {
		t.Fatalf("parallel allocation not isolated: %+v %+v %v", first, second, err)
	}
	if _, _, err = m.EnsureTask(ctx, root, "", "HEAD", "coding-a", "conversation-b"); !errors.Is(err, ErrBound) {
		t.Fatalf("cross-conversation task reused: %v", err)
	}
	if err = os.WriteFile(filepath.Join(first.Path, "go.mod"), []byte("module example.test/worker-a\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(first.Path, "worker_test.go"), []byte("package worker\nimport \"testing\"\nfunc TestCodingResult(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, first.Path, "add", "go.mod", "worker_test.go")
	gitTest(t, first.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "completed task a")
	checkpoint, err := m.PrepareCheckpoint(ctx, root, first.ID, "coding-a", "conversation-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.PreviewApprovedMerge(ctx, root, first.ID, "coding-a", "conversation-a"); !errors.Is(err, ErrApprovalDenied) {
		t.Fatalf("unapproved task merge preview allowed: %v", err)
	}
	tested, err := m.RunCheckpointTest(ctx, root, first.ID, "coding-a", "conversation-a", checkpoint.Revision, "go")
	if err != nil || len(tested.Tests) != 1 || !tested.Tests[0].Passed {
		t.Fatalf("real Go tests not verified: %+v %v", tested, err)
	}
	recovered, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	reattached, newAllocation, err := recovered.EnsureTask(ctx, root, "", "HEAD", "coding-a", "conversation-a")
	if err != nil || newAllocation || reattached.ID != first.ID {
		t.Fatalf("restart duplicated task allocation: %+v %v", reattached, err)
	}
	if _, err = recovered.ApplyApprovedMerge(ctx, root, first.ID, "coding-a", "conversation-a", "0000000000000000000000000000000000000000000000000000000000000000", "reviewer"); !errors.Is(err, ErrApprovalDenied) {
		t.Fatalf("merge without approval accepted: %v", err)
	}
	approved, err := recovered.ApproveCheckpoint(ctx, root, first.ID, "coding-a", "conversation-a", checkpoint.Revision, "reviewer")
	if err != nil || approved.ApprovedAt == nil {
		t.Fatalf("approved checkpoint not persisted: %+v %v", approved, err)
	}
	plan, err := recovered.PreviewApprovedMerge(ctx, root, first.ID, "coding-a", "conversation-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = recovered.ApplyApprovedMerge(ctx, root, first.ID, "coding-a", "conversation-a", plan.PlanID, "intruder"); !errors.Is(err, ErrApprovalDenied) {
		t.Fatalf("wrong merge actor allowed: %v", err)
	}
	merged, err := recovered.ApplyApprovedMerge(ctx, root, first.ID, "coding-a", "conversation-a", plan.PlanID, "reviewer")
	if err != nil || merged.Result != "applied" || gitTest(t, root, "rev-parse", "HEAD") != plan.SourceHead {
		t.Fatalf("approved task did not fast-forward: %+v %v", merged, err)
	}
	if _, err = recovered.ApplyApprovedMerge(ctx, root, first.ID, "coding-a", "conversation-a", plan.PlanID, "reviewer"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("merged task replayed: %v", err)
	}
	if err = os.WriteFile(filepath.Join(second.Path, "go.mod"), []byte("module example.test/worker-b\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(second.Path, "worker_test.go"), []byte("package worker\nimport \"testing\"\nfunc TestSecond(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, second.Path, "add", "go.mod", "worker_test.go")
	gitTest(t, second.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-m", "completed task b")
	checkpointB, err := recovered.PrepareCheckpoint(ctx, root, second.ID, "coding-b", "conversation-a")
	if err != nil {
		t.Fatal(err)
	}
	testedB, err := recovered.RunCheckpointTest(ctx, root, second.ID, "coding-b", "conversation-a", checkpointB.Revision, "go")
	if err != nil || !testedB.Tests[len(testedB.Tests)-1].Passed {
		t.Fatalf("second worker test failed: %v", err)
	}
	if _, err = recovered.ApproveCheckpoint(ctx, root, second.ID, "coding-b", "conversation-a", checkpointB.Revision, "reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err = recovered.PreviewApprovedMerge(ctx, root, second.ID, "coding-b", "conversation-a"); !errors.Is(err, ErrMergeNotFastForward) {
		t.Fatalf("diverged concurrent worker should require manual reconciliation: %v", err)
	}
}
