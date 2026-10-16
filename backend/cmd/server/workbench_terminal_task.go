package main

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/workbenchterminal"
)

type terminalWorktreeReader interface {
	ListTaskWorktrees(context.Context, string) ([]gitworktree.Entry, error)
}

func workbenchTerminalTaskVerifier(reader terminalWorktreeReader) workbenchterminal.TaskVerifier {
	if reader == nil {
		return nil
	}
	return func(ctx context.Context, workspaceURI, taskID, conversationID string) error {
		if workspaceURI == "" || !gitworktree.ValidBinding(taskID, conversationID) {
			return workbenchterminal.ErrTaskBinding
		}
		entries, err := reader.ListTaskWorktrees(ctx, workspaceURI)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Managed && entry.WorkspaceURI == workspaceURI && entry.TaskBinding != nil && entry.TaskBinding.TaskID == taskID && entry.TaskBinding.ConversationID == conversationID {
				return nil
			}
		}
		return errors.Join(workbenchterminal.ErrTaskBinding, gitworktree.ErrBindingMismatch)
	}
}
