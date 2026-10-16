package main

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/interaction"
)

type fakeCodingTerminalReader struct {
	calls        int
	task         string
	workspaceURI string
	err          error
}

func (f *fakeCodingTerminalReader) ReadCodingTask(_ context.Context, _ chat.SkillScope, uri, task string) (interaction.CodingTerminalDiagnostics, error) {
	f.calls++
	f.task = task
	f.workspaceURI = uri
	if f.err != nil {
		return interaction.CodingTerminalDiagnostics{}, f.err
	}
	return interaction.CodingTerminalDiagnostics{State: "recorded", VerificationKind: "terminal-lifecycle-metadata-not-test-proof", Total: 2, Exited: 1, UnverifiedActive: 1}, nil
}
func TestCodingTerminalEvidenceAttachedOnlyAfterScopedWorktreeReview(t *testing.T) {
	base := sampleReviewGit()
	terminal := &fakeCodingTerminalReader{}
	runtime := &chatMultiAgentRuntime{worktreeReviewer: codingWorktreeReviewerService{git: base}, terminalReader: terminal}
	scope := chat.SkillScope{ConversationID: "conv-a", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"workspaceRootUri": "amitia://workspace/@main/"}}}
	snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded}}}
	runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
	got := snapshot.Assignments[0].WorktreeReview
	if got == nil || got.Terminal == nil || got.Terminal.State != "recorded" || terminal.calls != 1 || terminal.task != "task-a" || terminal.workspaceURI != base.entries[0].WorkspaceURI {
		t.Fatalf("terminal diagnostics not scoped to reviewed worktree: %+v calls=%d", got, terminal.calls)
	}
}
func TestCodingTerminalEvidenceUnavailableAndNoLeak(t *testing.T) {
	for _, tc := range []struct {
		name      string
		hasScope  bool
		readError bool
	}{
		{"missing-workspace", false, false}, {"journal-error", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := &fakeCodingTerminalReader{}
			if tc.readError {
				term.err = errors.New("journal error")
			}
			runtime := &chatMultiAgentRuntime{worktreeReviewer: codingWorktreeReviewerService{git: sampleReviewGit()}, terminalReader: term}
			meta := map[string]any{}
			if tc.hasScope {
				meta["workspaceRootUri"] = "amitia://workspace/@main/"
			}
			scope := chat.SkillScope{ConversationID: "conv-a", ExecContext: &execution.ExecutionContext{Metadata: meta}}
			snapshot := interaction.CoordinationSnapshot{SourceWorkspaceURI: "amitia://workspace/@main/", Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "task-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded}}}
			runtime.attachCodingWorktreeEvidence(context.Background(), scope, &snapshot)
			got := snapshot.Assignments[0].WorktreeReview
			if !tc.hasScope {
				if got == nil || got.Terminal != nil || term.calls != 0 {
					t.Fatalf("unscoped diagnostic leaked: %+v", got)
				}
				return
			}
			if got == nil || got.Terminal == nil || got.Terminal.State != "unavailable" || got.Terminal.ErrorCode != "TERMINAL_DIAGNOSTICS_UNAVAILABLE" || term.calls != 1 {
				t.Fatalf("failure concealed: %+v", got)
			}
		})
	}
}
