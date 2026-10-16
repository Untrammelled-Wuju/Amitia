package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/interaction"
)

func TestCodingWorkerReviewGateAndSummaryAreFailClosed(t *testing.T) {
	runtime := &chatMultiAgentRuntime{worktreeReviewer: codingWorktreeReviewerService{git: sampleReviewGit()}}
	scope := chat.SkillScope{ConversationID: "conv-a", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workspaceRootUri": "amitia://workspace/@main/"}}}
	snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{
		{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded},
		{AssignmentID: "task-b", WorkerRole: "coding_worker", Status: interaction.AssignmentRunning},
	}}
	runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
	if snapshot.VerificationSummary == nil || snapshot.VerificationSummary.Total != 2 || snapshot.VerificationSummary.Waiting != 1 || snapshot.VerificationSummary.Blocked != 1 || snapshot.VerificationSummary.AllMergesConfirmed {
		t.Fatalf("unverified workers considered complete: %+v", snapshot.VerificationSummary)
	}
	gate := snapshot.Assignments[0].WorktreeReview.VerificationGate
	if gate == nil || gate.ConfirmedMerged || gate.EligibleForMerge || gate.Code != "VERIFIED_TEST_REQUIRED" {
		t.Fatalf("missing real test evidence accepted: %+v", gate)
	}
}

func TestCodingWorkerReviewGateRejectsUnscopedResult(t *testing.T) {
	runtime := &chatMultiAgentRuntime{worktreeReviewer: codingWorktreeReviewerService{git: sampleReviewGit()}}
	snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded}}}
	runtime.attachCodingWorktreeEvidence(context.Background(), chat.SkillScope{ConversationID: "conv-b", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workspaceRootUri": "amitia://workspace/@wrong/"}}}, &snapshot)
	review := snapshot.Assignments[0].WorktreeReview
	if review == nil || review.VerificationGate == nil || review.VerificationGate.EligibleForMerge || review.VerificationGate.Code != "REVIEW_INCOMPLETE_OR_UNAVAILABLE" {
		t.Fatalf("unscoped worktree marked usable: %+v", review)
	}
}
