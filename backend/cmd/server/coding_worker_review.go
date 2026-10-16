package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/interaction"
	workspacegit "github.com/u-ai/backend/internal/workspace/git"
)

type codingWorktreeReviewer interface {
	ReviewCodingTask(context.Context, string, string, string) (interaction.CodingWorktreeReview, error)
}

type codingWorktreeReviewGit interface {
	ListTaskWorktrees(context.Context, string) ([]gitworktree.Entry, error)
	ReviewTaskWorktree(context.Context, workspacegit.WorktreeReviewRequest) (*workspacegit.WorktreeReviewResult, error)
}

type codingWorktreeCheckpointReader interface {
	ReadTaskCheckpoint(context.Context, workspacegit.WorktreeCheckpointRequest) (*gitworktree.ReviewCheckpoint, error)
}

type codingWorktreeMergeReader interface {
	ReadTaskMergeAudit(context.Context, workspacegit.WorktreeMergeRequest) (*gitworktree.MergeAudit, error)
}

type codingWorktreeReviewerService struct {
	git           codingWorktreeReviewGit
	services      *AppServices
	localDeviceID string
	mesh          *codingWorkerMeshAllocator
}

var errCodingReviewUnavailable = errors.New("coding worktree review unavailable")

func (r codingWorktreeReviewerService) ReviewCodingTask(ctx context.Context, sourceWorkspaceURI, taskID, conversationID string) (interaction.CodingWorktreeReview, error) {
	if scope, ok := coordination.FromContext(ctx); ok && scope.TargetDeviceID != "" && scope.TargetDeviceID != r.localDeviceID {
		if r.mesh == nil && r.services != nil {
			r.mesh = newCodingWorkerWorktreeAllocator(func() *AppServices { return r.services }, nil, r.localDeviceID).mesh
		}
		if r.mesh == nil {
			return interaction.CodingWorktreeReview{}, errCodingReviewUnavailable
		}
		return r.mesh.ReviewCodingTask(ctx, sourceWorkspaceURI, taskID, conversationID)
	}
	if r.git == nil || !strings.HasPrefix(sourceWorkspaceURI, "amitia://workspace/@") ||
		strings.TrimSpace(taskID) == "" || strings.TrimSpace(conversationID) == "" ||
		!gitworktree.ValidBinding(taskID, conversationID) {
		return interaction.CodingWorktreeReview{}, errCodingReviewUnavailable
	}
	entries, err := r.git.ListTaskWorktrees(ctx, sourceWorkspaceURI)
	if err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	var selected *gitworktree.Entry
	for i := range entries {
		entry := &entries[i]
		if !entry.Managed || entry.TaskBinding == nil || entry.TaskBinding.TaskID != taskID || entry.TaskBinding.ConversationID != conversationID {
			continue
		}
		if selected != nil || entry.WorkspaceURI == "" || entry.ID == "" {
			return interaction.CodingWorktreeReview{}, errCodingReviewUnavailable
		}
		selected = entry
	}
	if selected == nil {
		return interaction.CodingWorktreeReview{}, gitworktree.ErrNotManaged
	}
	reviewed, err := r.git.ReviewTaskWorktree(ctx, workspacegit.WorktreeReviewRequest{
		WorkspaceURI: sourceWorkspaceURI, ID: selected.ID,
		TaskID: taskID, ConversationID: conversationID, ExpectedHead: selected.Head,
	})
	if err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	if reviewed == nil || reviewed.Status == nil || reviewed.Diff == nil || reviewed.Entry.TaskBinding == nil ||
		reviewed.Entry.ID != selected.ID || reviewed.Entry.WorkspaceURI != selected.WorkspaceURI ||
		reviewed.Entry.Head != selected.Head || reviewed.Entry.TaskBinding.TaskID != taskID ||
		reviewed.Entry.TaskBinding.ConversationID != conversationID {
		return interaction.CodingWorktreeReview{}, errCodingReviewUnavailable
	}
	incomplete := reviewed.ReviewIncomplete || reviewed.Diff.Truncated || reviewed.Status.Truncated || !reviewed.BaselineKnown || reviewed.UntrackedCount > 0 || reviewed.ConflictedCount > 0
	state := "ready"
	if incomplete {
		state = "incomplete"
	}
	evidence := interaction.CodingWorktreeReview{
		State: state, WorktreeID: selected.ID, WorkspaceURI: selected.WorkspaceURI,
		BaselineHead: reviewed.Entry.TaskBinding.BaseHead, CurrentHead: reviewed.Entry.Head,
		FilesChanged: reviewed.Diff.Stats.FilesChanged, Insertions: reviewed.Diff.Stats.Insertions,
		Deletions: reviewed.Diff.Stats.Deletions, UntrackedCount: reviewed.UntrackedCount,
		ConflictedCount: reviewed.ConflictedCount, ReviewIncomplete: incomplete,
		RequiresApproval: true, TestState: "not_run", ApprovalState: "pending",
	}
	if reader, ok := r.git.(codingWorktreeCheckpointReader); ok && !incomplete {
		checkpoint, checkErr := reader.ReadTaskCheckpoint(ctx, workspacegit.WorktreeCheckpointRequest{
			WorkspaceURI: sourceWorkspaceURI, ID: selected.ID, TaskID: taskID, ConversationID: conversationID,
		})
		if checkErr == nil && checkpoint != nil && checkpoint.Head == selected.Head && checkpoint.BaseHead == reviewed.Entry.TaskBinding.BaseHead && checkpoint.TaskID == taskID && checkpoint.ConversationID == conversationID {
			evidence.CheckpointRevision = checkpoint.Revision
			if len(checkpoint.Tests) > 0 {
				latest := checkpoint.Tests[len(checkpoint.Tests)-1]
				evidence.TestProfile = latest.Profile
				evidence.TestExitCode = latest.ExitCode
				evidence.TestOutputDigest = latest.OutputDigest
				if !latest.StartedAt.IsZero() {
					evidence.TestStartedAt = latest.StartedAt.UTC().Format(time.RFC3339Nano)
				}
				if gitworktree.VerifiedTestEvidenceComplete(latest, checkpoint.Revision) {
					evidence.TestState = "passed"
				} else {
					evidence.TestState = "failed"
				}
			}
			if checkpoint.ApprovedAt != nil && evidence.TestState == "passed" {
				evidence.ApprovalState = "approved"
				evidence.MergeState = "unavailable"
				if reader, ok := r.git.(codingWorktreeMergeReader); ok {
					audit, auditErr := reader.ReadTaskMergeAudit(ctx, workspacegit.WorktreeMergeRequest{
						WorkspaceURI: sourceWorkspaceURI, ID: selected.ID,
						TaskID: taskID, ConversationID: conversationID,
					})
					if errors.Is(auditErr, gitworktree.ErrCheckpointRequired) {
						evidence.MergeState = "not_attempted"
					} else if auditErr == nil && audit != nil && audit.Plan.WorktreeID == selected.ID &&
						audit.Plan.TaskID == taskID && audit.Plan.ConversationID == conversationID &&
						audit.Plan.SourceHead == checkpoint.Head && audit.Plan.Revision == checkpoint.Revision &&
						audit.Plan.ApprovedBy == checkpoint.ApprovedBy && audit.Actor == checkpoint.ApprovedBy {
						evidence.MergeState = audit.Result
						evidence.MergeTargetBranch = audit.Plan.TargetBranch
						if audit.Result == "applied" && audit.AppliedHead == checkpoint.Head {
							evidence.MergeAppliedHead = audit.AppliedHead
						} else if audit.Result == "applied" {
							evidence.MergeState = "unavailable"
						}
					}
				}
			}
		}
	}
	return evidence, nil
}

func (m *chatMultiAgentRuntime) attachCodingWorktreeEvidence(ctx context.Context, scope chat.SkillScope, snapshot *interaction.CoordinationSnapshot) {
	if m == nil || snapshot == nil {
		return
	}
	sourceURI := snapshot.SourceWorkspaceURI
	authorizedURI := ""
	if scope.ExecContext != nil && scope.ExecContext.Metadata != nil {
		authorizedURI, _ = scope.ExecContext.Metadata["workspaceRootUri"].(string)
	}
	authorized := sourceURI != "" && strings.TrimSuffix(sourceURI, "/") == strings.TrimSuffix(authorizedURI, "/")
	for i := range snapshot.Assignments {
		assignment := &snapshot.Assignments[i]
		if assignment.WorkerRole != "coding_worker" {
			continue
		}
		evidence := &interaction.CodingWorktreeReview{State: "pending", RequiresApproval: true}
		assignment.WorktreeReview = evidence
		evidence.VerificationGate = ptrCodingVerification(interaction.EvaluateCodingWorkerVerification(assignment.Status, evidence))
		if !assignment.Status.IsTerminal() {
			continue
		}
		evidence.State = "unavailable"
		evidence.ReviewIncomplete = true
		if !authorized || m.worktreeReviewer == nil {
			evidence.ErrorCode = "WORKSPACE_SCOPE_UNAVAILABLE"
			evidence.VerificationGate = ptrCodingVerification(interaction.EvaluateCodingWorkerVerification(assignment.Status, evidence))
			continue
		}
		reviewCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		reviewed, err := m.worktreeReviewer.ReviewCodingTask(reviewCtx, sourceURI, assignment.AssignmentID, scope.ConversationID)
		cancel()
		if err != nil || (reviewed.State != "ready" && reviewed.State != "incomplete") || reviewed.WorktreeID == "" || reviewed.WorkspaceURI == "" {
			evidence.ErrorCode = "WORKTREE_REVIEW_UNAVAILABLE"
			evidence.VerificationGate = ptrCodingVerification(interaction.EvaluateCodingWorkerVerification(assignment.Status, evidence))
			continue
		}
		reviewed.RequiresApproval = true
		if m.terminalReader != nil {
			diagnostic, diagErr := m.terminalReader.ReadCodingTask(ctx, scope, reviewed.WorkspaceURI, assignment.AssignmentID)
			if diagErr != nil {
				reviewed.Terminal = &interaction.CodingTerminalDiagnostics{State: "unavailable", VerificationKind: "terminal-lifecycle-metadata-not-test-proof", ErrorCode: "TERMINAL_DIAGNOSTICS_UNAVAILABLE"}
			} else {
				reviewed.Terminal = &diagnostic
			}
		}
		reviewed.ReviewIncomplete = reviewed.ReviewIncomplete || reviewed.State != "ready"
		*evidence = reviewed
		evidence.VerificationGate = ptrCodingVerification(interaction.EvaluateCodingWorkerVerification(assignment.Status, evidence))
	}
	snapshot.VerificationSummary = interaction.SummarizeCodingWorkerEvidence(snapshot.Assignments)
}

func ptrCodingVerification(gate interaction.CodingWorkerVerificationGate) *interaction.CodingWorkerVerificationGate {
	return &gate
}
