package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/interaction"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

type fakeReviewGit struct {
	checkpoint      *gitworktree.ReviewCheckpoint
	mergeAudit      *gitworktree.MergeAudit
	mergeAuditErr   error
	mergeAuditCalls int
	checkpointErr   error
	checkpointCalls int
	entries         []gitworktree.Entry
	review          *workspacegit.WorktreeReviewResult
	request         workspacegit.WorktreeReviewRequest
	err             error
	calls           int
}

func (f *fakeReviewGit) ListTaskWorktrees(context.Context, string) ([]gitworktree.Entry, error) {
	return f.entries, f.err
}
func (f *fakeReviewGit) ReviewTaskWorktree(_ context.Context, req workspacegit.WorktreeReviewRequest) (*workspacegit.WorktreeReviewResult, error) {
	f.calls++
	f.request = req
	return f.review, f.err
}

func (f *fakeReviewGit) ReadTaskCheckpoint(_ context.Context, req workspacegit.WorktreeCheckpointRequest) (*gitworktree.ReviewCheckpoint, error) {
	f.checkpointCalls++
	if len(f.entries) > 0 && (req.TaskID != f.entries[0].TaskBinding.TaskID || req.ConversationID != f.entries[0].TaskBinding.ConversationID) {
		return nil, gitworktree.ErrBindingMismatch
	}
	return f.checkpoint, f.checkpointErr
}

func (f *fakeReviewGit) ReadTaskMergeAudit(_ context.Context, req workspacegit.WorktreeMergeRequest) (*gitworktree.MergeAudit, error) {
	f.mergeAuditCalls++
	if len(f.entries) > 0 && (req.ID != f.entries[0].ID || req.TaskID != f.entries[0].TaskBinding.TaskID || req.ConversationID != f.entries[0].TaskBinding.ConversationID) {
		return nil, gitworktree.ErrBindingMismatch
	}
	if f.mergeAuditErr != nil {
		return nil, f.mergeAuditErr
	}
	if f.mergeAudit == nil {
		return nil, gitworktree.ErrCheckpointRequired
	}
	return f.mergeAudit, nil
}

func sampleReviewGit() *fakeReviewGit {
	binding := &gitworktree.TaskBinding{TaskID: "task-a", ConversationID: "conv-a", BaseHead: "baseline"}
	entry := gitworktree.Entry{ID: "0123456789abcdef0123456789abcdef", Head: "new-head", WorkspaceURI: "amitia://workspace/@isolated/", Managed: true, TaskBinding: binding}
	return &fakeReviewGit{entries: []gitworktree.Entry{entry}, review: &workspacegit.WorktreeReviewResult{
		Entry: entry, Status: &workspacegit.GitStatusResult{}, Diff: &workspacegit.GitDiffResult{Stats: workspacegit.GitDiffStats{FilesChanged: 3, Insertions: 12}}, BaselineKnown: true,
	}}
}

func TestCodingReviewReturnsScopedSummaryWithoutRawDiff(t *testing.T) {
	f := sampleReviewGit()
	got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.State != "ready" || got.FilesChanged != 3 || !got.RequiresApproval || f.request.ExpectedHead != "new-head" || f.calls != 1 {
		t.Fatalf("invalid summary: %+v %v %+v", got, err, f.request)
	}
}

func TestCodingReviewRejectsOtherTaskAndConversation(t *testing.T) {
	for _, tc := range []struct{ task, conv string }{{"other", "conv-a"}, {"task-a", "other"}, {"", "conv-a"}} {
		f := sampleReviewGit()
		_, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", tc.task, tc.conv)
		if err == nil || f.calls != 0 {
			t.Fatalf("unauthorized review dispatched: %v", err)
		}
	}
}

func TestCodingReviewRejectsAmbiguousAndStaleResults(t *testing.T) {
	f := sampleReviewGit()
	f.entries = append(f.entries, f.entries[0])
	if _, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a"); err == nil || f.calls != 0 {
		t.Fatal("ambiguous worktree selected")
	}
	f = sampleReviewGit()
	f.review.Entry.Head = "different"
	if _, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a"); !errors.Is(err, errCodingReviewUnavailable) {
		t.Fatal("stale review accepted")
	}
}

func TestCodingReviewMarksPartialResults(t *testing.T) {
	f := sampleReviewGit()
	f.review.ReviewIncomplete = true
	got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.State != "incomplete" || !got.ReviewIncomplete || !got.RequiresApproval {
		t.Fatal("partial review claimed ready")
	}
}

func TestCodingReviewPendingCannotClaimApproval(t *testing.T) {
	pending := interaction.CodingWorktreeReview{State: "pending", RequiresApproval: true}
	if pending.State == "ready" || !pending.RequiresApproval {
		t.Fatal("pending review invalid")
	}
}

type fakeEvidenceReviewer struct {
	calls            int
	seenTask         string
	seenConversation string
	evidence         interaction.CodingWorktreeReview
	err              error
}

func (f *fakeEvidenceReviewer) ReviewCodingTask(_ context.Context, _, taskID, conversationID string) (interaction.CodingWorktreeReview, error) {
	f.calls++
	f.seenTask = taskID
	f.seenConversation = conversationID
	return f.evidence, f.err
}

func TestCodingEvidenceOnlyAfterTerminalAndMatchingWorkspace(t *testing.T) {
	reader := &fakeEvidenceReviewer{evidence: interaction.CodingWorktreeReview{State: "ready", WorktreeID: "id-a", WorkspaceURI: "amitia://workspace/@isolated/"}}
	runtime := &chatMultiAgentRuntime{worktreeReviewer: reader}
	snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{
		{AssignmentID: "task-running", WorkerRole: "coding_worker", Status: interaction.AssignmentRunning},
		{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded},
		{AssignmentID: "other-role", WorkerRole: "research_worker", Status: interaction.AssignmentSucceeded},
	}}
	scope := chat.SkillScope{ConversationID: "conv-a", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workspaceRootUri": "amitia://workspace/@main/"}}}
	runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
	if reader.calls != 1 || reader.seenTask != "task-a" || reader.seenConversation != "conv-a" || snapshot.Assignments[0].WorktreeReview.State != "pending" || snapshot.Assignments[1].WorktreeReview.State != "ready" || !snapshot.Assignments[1].WorktreeReview.RequiresApproval || snapshot.Assignments[2].WorktreeReview != nil {
		t.Fatalf("evidence not scoped: %+v", snapshot.Assignments)
	}
	reader.calls = 0
	scope.ExecContext.Metadata["workspaceRootUri"] = "amitia://workspace/@different/"
	runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
	if reader.calls != 0 || snapshot.Assignments[1].WorktreeReview.State != "unavailable" || snapshot.Assignments[1].WorktreeReview.ErrorCode != "WORKSPACE_SCOPE_UNAVAILABLE" {
		t.Fatalf("mismatched workspaces queried reviewer: %+v", snapshot.Assignments[1])
	}
}

func TestCodingEvidenceFailedReadNeverClaimsVerified(t *testing.T) {
	reader := &fakeEvidenceReviewer{err: errors.New("disk error")}
	runtime := &chatMultiAgentRuntime{worktreeReviewer: reader}
	snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded}}}
	scope := chat.SkillScope{ConversationID: "conv-a", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workspaceRootUri": "amitia://workspace/@main/"}}}
	runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
	reviewed := snapshot.Assignments[0].WorktreeReview
	if reviewed == nil || reviewed.State != "unavailable" || !reviewed.ReviewIncomplete || !reviewed.RequiresApproval || reviewed.ErrorCode != "WORKTREE_REVIEW_UNAVAILABLE" || reviewed.WorktreeID != "" {
		t.Fatalf("fabricated success despite review failure: %+v", reviewed)
	}
}

func TestCodingReviewConflictsAndUntrackedFilesAreIncomplete(t *testing.T) {
	for _, kind := range []string{"conflicted", "untracked", "truncated", "missing baseline"} {
		f := sampleReviewGit()
		switch kind {
		case "conflicted":
			f.review.ConflictedCount = 1
		case "untracked":
			f.review.UntrackedCount = 1
		case "truncated":
			f.review.Diff.Truncated = true
		case "missing baseline":
			f.review.BaselineKnown = false
		}
		got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
		if err != nil || got.State != "incomplete" || !got.ReviewIncomplete || !got.RequiresApproval {
			t.Fatalf("incomplete condition %s claimed ready: %+v %v", kind, got, err)
		}
	}
}

func TestCodingEvidenceRejectsMalformedReviewerResponse(t *testing.T) {
	reader := &fakeEvidenceReviewer{evidence: interaction.CodingWorktreeReview{State: "ready"}}
	runtime := &chatMultiAgentRuntime{worktreeReviewer: reader}
	snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{
		{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded},
	}}
	scope := chat.SkillScope{ConversationID: "conv-a", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workspaceRootUri": "amitia://workspace/@main/"}}}
	runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
	evidence := snapshot.Assignments[0].WorktreeReview
	if evidence == nil || evidence.State != "unavailable" || evidence.WorktreeID != "" {
		t.Fatalf("malformed reviewer reported ready: %+v", evidence)
	}
}

func verifiedReviewTestFixture(revision string) gitworktree.VerifiedTest {
	return gitworktree.VerifiedTest{
		Profile: "go", Revision: revision, StartedAt: time.Now().UTC(),
		ExitCode: 0, Passed: true, OutputDigest: strings.Repeat("a", 64),
	}
}

func TestCodingReviewCheckpointEvidenceMustMatchExactTaskAndHead(t *testing.T) {
	f := sampleReviewGit()
	f.checkpoint = &gitworktree.ReviewCheckpoint{Version: 1, WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a", Head: "new-head", BaseHead: "baseline", Revision: "revision-a", Tests: []gitworktree.VerifiedTest{verifiedReviewTestFixture("revision-a")}}
	result, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || result.TestState != "passed" || result.ApprovalState != "pending" || result.CheckpointRevision != "revision-a" {
		t.Fatalf("invalid checkpoint evidence: %+v %v", result, err)
	}
	f.checkpoint.Head = "different-head"
	result, err = (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || result.TestState != "not_run" || result.ApprovalState != "pending" {
		t.Fatalf("stale evidence accepted: %+v %v", result, err)
	}
}

func TestCodingReviewProjectsVerifiedTestEvidenceForParentAgent(t *testing.T) {
	f := sampleReviewGit()
	revision := strings.Repeat("a", 64)
	verified := verifiedReviewTestFixture(revision)
	f.checkpoint = &gitworktree.ReviewCheckpoint{
		Version: 1, WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a",
		Head: "new-head", BaseHead: "baseline", Revision: revision, Tests: []gitworktree.VerifiedTest{verified},
	}
	got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.TestState != "passed" || got.TestProfile != verified.Profile ||
		got.TestOutputDigest != verified.OutputDigest || got.TestStartedAt == "" ||
		got.TestExitCode != 0 || !interaction.EvaluateCodingWorkerVerification(interaction.AssignmentSucceeded, &got).ManualActionRequired {
		t.Fatalf("verified test evidence not correctly projected: %+v %v", got, err)
	}
	got.TestOutputDigest = ""
	if gate := interaction.EvaluateCodingWorkerVerification(interaction.AssignmentSucceeded, &got); gate.Code != "VERIFIED_TEST_REQUIRED" {
		t.Fatalf("missing evidence was accepted: %+v", gate)
	}
}

func TestCodingReviewFailedTestsCannotClaimApproval(t *testing.T) {
	f := sampleReviewGit()
	now := time.Now().UTC()
	f.checkpoint = &gitworktree.ReviewCheckpoint{Version: 1, WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a", Head: "new-head", BaseHead: "baseline", Revision: "revision-a", ApprovedAt: &now, Tests: []gitworktree.VerifiedTest{verifiedReviewTestFixture("revision-a"), {Profile: "go-race", Revision: "revision-a", ExitCode: 1, Passed: false}}}
	result, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || result.TestState != "failed" || result.ApprovalState != "pending" {
		t.Fatalf("failed test claimed approval: %+v %v", result, err)
	}
}

func TestCodingReviewMergeAuditOnlyAfterVerifiedApproval(t *testing.T) {
	f := sampleReviewGit()
	now := time.Now().UTC()
	f.checkpoint = &gitworktree.ReviewCheckpoint{Version: 1, WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a", Head: "new-head", BaseHead: "baseline", Revision: "revision-a", ApprovedAt: &now, ApprovedBy: "local-owner", Tests: []gitworktree.VerifiedTest{verifiedReviewTestFixture("revision-a")}}
	f.mergeAudit = &gitworktree.MergeAudit{Version: 1, Plan: gitworktree.MergeProposal{WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a", SourceHead: "new-head", Revision: "revision-a", TargetBranch: "main", ApprovedBy: "local-owner"}, Actor: "local-owner", Result: "applied", AppliedHead: "new-head"}
	got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.MergeState != "applied" || got.MergeAppliedHead != "new-head" || got.MergeTargetBranch != "main" || f.mergeAuditCalls != 1 {
		t.Fatalf("merge evidence missing: %+v %v", got, err)
	}
	f.mergeAudit.Plan.TaskID = "other-task"
	got, err = (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.MergeState != "unavailable" || got.MergeAppliedHead != "" {
		t.Fatalf("forged merge audit accepted: %+v %v", got, err)
	}
	f.mergeAudit.Plan.TaskID = "task-a"
	f.mergeAudit = nil
	got, err = (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.MergeState != "not_attempted" {
		t.Fatalf("missing audit falsely treated as merged: %+v %v", got, err)
	}
}

func TestCodingReviewNeverFetchesMergeWithoutApproval(t *testing.T) {
	f := sampleReviewGit()
	f.checkpoint = &gitworktree.ReviewCheckpoint{Version: 1, WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a", Head: "new-head", BaseHead: "baseline", Revision: "revision-a", Tests: []gitworktree.VerifiedTest{verifiedReviewTestFixture("revision-a")}}
	got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || got.MergeState != "" || f.mergeAuditCalls != 0 {
		t.Fatalf("unapproved merge queried: %+v %v", got, err)
	}
}

func TestCodingReviewRejectsIncompleteSuccessEvidence(t *testing.T) {
	cases := map[string]func(*gitworktree.VerifiedTest){
		"missing digest":    func(e *gitworktree.VerifiedTest) { e.OutputDigest = "" },
		"nonzero exit":      func(e *gitworktree.VerifiedTest) { e.ExitCode = 1 },
		"unverified runner": func(e *gitworktree.VerifiedTest) { e.Profile = "shell" },
		"output truncated":  func(e *gitworktree.VerifiedTest) { e.OutputTruncated = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := sampleReviewGit()
			now := time.Now().UTC()
			evidence := verifiedReviewTestFixture("revision-a")
			mutate(&evidence)
			f.checkpoint = &gitworktree.ReviewCheckpoint{
				Version: 1, WorktreeID: f.entries[0].ID, TaskID: "task-a", ConversationID: "conv-a",
				Head: "new-head", BaseHead: "baseline", Revision: "revision-a", ApprovedAt: &now,
				ApprovedBy: "local-owner", Tests: []gitworktree.VerifiedTest{evidence},
			}
			f.mergeAudit = &gitworktree.MergeAudit{Result: "applied", AppliedHead: "new-head"}
			got, err := (codingWorktreeReviewerService{git: f}).ReviewCodingTask(context.Background(), "amitia://workspace/@main/", "task-a", "conv-a")
			if err != nil || got.TestState != "failed" || got.ApprovalState != "pending" || got.MergeState != "" || f.mergeAuditCalls != 0 {
				t.Fatalf("invalid evidence leaked as approved: %+v %v", got, err)
			}
		})
	}
}
