package main

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/workbenchterminal"
)

type codingTaskTerminalReader interface {
	ReadCodingTask(context.Context, chat.SkillScope, string, string) (interaction.CodingTerminalDiagnostics, error)
}

type terminalReceiptJournal interface {
	ListRecorded(string, string, string) ([]workbenchterminal.TaskReceipt, error)
}

type codingTaskTerminalService struct {
	journal terminalReceiptJournal
}

var errTerminalScopeUnavailable = errors.New("terminal diagnostics require an authenticated trusted local UI session")

func newCodingTaskTerminalReader() codingTaskTerminalReader {
	journal, err := workbenchterminal.OpenDefaultTaskReceiptJournal()
	if err != nil {
		return codingTaskTerminalService{}
	}
	return codingTaskTerminalService{journal: journal}
}

func (s codingTaskTerminalService) ReadCodingTask(ctx context.Context, scope chat.SkillScope, workspaceURI, taskID string) (interaction.CodingTerminalDiagnostics, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.PrincipalType != auth.PrincipalLocalUI || !actor.IsLocalTrusted ||
		actor.SpaceID.String() != scope.SpaceID || actor.SessionID == "" ||
		scope.ConversationID == "" ||
		taskID == "" || workspaceURI == "" || s.journal == nil {
		return interaction.CodingTerminalDiagnostics{}, errTerminalScopeUnavailable
	}
	owner := actor.SpaceID.String() + ":" + actor.DeviceID.String() + ":" + actor.SessionID + ":" + scope.ConversationID
	records, err := s.journal.ListRecorded(owner, scope.ConversationID, taskID)
	if err != nil {
		return interaction.CodingTerminalDiagnostics{}, err
	}
	diagnostic := interaction.CodingTerminalDiagnostics{State: "none", VerificationKind: "terminal-lifecycle-metadata-not-test-proof"}
	for _, record := range records {
		if record.WorkspaceURI != workspaceURI || record.ConversationID != scope.ConversationID || record.TaskID != taskID {
			continue
		}
		if diagnostic.Total == 0 {
			diagnostic.LatestState = record.State
			if record.State == "running" {
				diagnostic.LatestState = "unverified"
			}
			if record.ExitCode != nil {
				code := *record.ExitCode
				diagnostic.LastExitCode = &code
			}
		}
		diagnostic.Total++
		switch record.State {
		case "exited":
			diagnostic.Exited++
		case "closed":
			diagnostic.Closed++
		case "revoked":
			diagnostic.Revoked++
		case "running", "interrupted":
			diagnostic.UnverifiedActive++
		}
	}
	if diagnostic.Total > 0 {
		diagnostic.State = "recorded"
	}
	return diagnostic, nil
}
