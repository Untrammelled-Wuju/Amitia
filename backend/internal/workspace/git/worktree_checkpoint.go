package git

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/gitworktree"
)

type WorktreeCheckpointRequest struct {
	WorkspaceURI   string `json:"workspaceUri"`
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	ConversationID string `json:"conversationId"`
	Revision       string `json:"revision,omitempty"`
	Profile        string `json:"profile,omitempty"`
}

func (c *GitController) checkpointManager(req WorktreeCheckpointRequest) (*gitworktree.Manager, string, error) {
	if req.WorkspaceURI == "" || len(req.ID) != 32 || !gitworktree.ValidBinding(req.TaskID, req.ConversationID) {
		return nil, "", gitworktree.ErrInvalidArgument
	}
	root, mount, err := c.resolveRepositoryPath(req.WorkspaceURI)
	if err != nil {
		return nil, "", err
	}
	if mount.ReadOnly {
		return nil, "", ErrGitOperationUnsupported
	}
	manager, err := gitworktree.New(c.roots.DataRoot())
	if err != nil {
		return nil, "", err
	}
	return manager, root, nil
}

func (c *GitController) WorktreeCheckpointPrepare(ctx context.Context, req WorktreeCheckpointRequest) (*gitworktree.ReviewCheckpoint, error) {
	review, err := c.WorktreeReview(ctx, WorktreeReviewRequest{WorkspaceURI: req.WorkspaceURI, ID: req.ID, TaskID: req.TaskID, ConversationID: req.ConversationID})
	if err != nil {
		return nil, err
	}
	if review.ReviewIncomplete {
		return nil, gitworktree.ErrApprovalDenied
	}
	manager, root, err := c.checkpointManager(req)
	if err != nil {
		return nil, err
	}
	return manager.PrepareCheckpoint(ctx, root, req.ID, req.TaskID, req.ConversationID)
}

func (c *GitController) WorktreeCheckpointRead(ctx context.Context, req WorktreeCheckpointRequest) (*gitworktree.ReviewCheckpoint, error) {
	manager, root, err := c.checkpointManager(req)
	if err != nil {
		return nil, err
	}
	return manager.ReadCheckpoint(ctx, root, req.ID, req.TaskID, req.ConversationID)
}

func (c *GitController) WorktreeCheckpointTest(ctx context.Context, req WorktreeCheckpointRequest) (*gitworktree.ReviewCheckpoint, error) {
	if req.Revision == "" || req.Profile == "" {
		return nil, gitworktree.ErrInvalidArgument
	}
	manager, root, err := c.checkpointManager(req)
	if err != nil {
		return nil, err
	}
	return manager.RunCheckpointTest(ctx, root, req.ID, req.TaskID, req.ConversationID, req.Revision, req.Profile)
}

func (c *GitController) WorktreeCheckpointApprove(ctx context.Context, req WorktreeCheckpointRequest, approver string) (*gitworktree.ReviewCheckpoint, error) {
	if req.Revision == "" {
		return nil, gitworktree.ErrInvalidArgument
	}
	review, err := c.WorktreeReview(ctx, WorktreeReviewRequest{WorkspaceURI: req.WorkspaceURI, ID: req.ID, TaskID: req.TaskID, ConversationID: req.ConversationID, ExpectedHead: ""})
	if err != nil {
		return nil, err
	}
	if review.ReviewIncomplete {
		return nil, gitworktree.ErrApprovalDenied
	}
	manager, root, err := c.checkpointManager(req)
	if err != nil {
		return nil, err
	}
	return manager.ApproveCheckpoint(ctx, root, req.ID, req.TaskID, req.ConversationID, req.Revision, approver)
}

var ErrWorktreeApprovalAuthorization = errors.New("task approval and execution require a trusted local UI session")
