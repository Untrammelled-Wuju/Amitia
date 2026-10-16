package git

import (
	"context"
	"github.com/u-ai/backend/internal/gitworktree"
)

type WorktreeMergeRequest struct {
	WorkspaceURI   string `json:"workspaceUri"`
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	ConversationID string `json:"conversationId"`
	PlanID         string `json:"planId,omitempty"`
}

func (c *GitController) WorktreeMergePreview(ctx context.Context, req WorktreeMergeRequest) (*gitworktree.MergeProposal, error) {
	manager, root, err := c.checkpointManager(WorktreeCheckpointRequest{WorkspaceURI: req.WorkspaceURI, ID: req.ID, TaskID: req.TaskID, ConversationID: req.ConversationID})
	if err != nil {
		return nil, err
	}
	return manager.PreviewApprovedMerge(ctx, root, req.ID, req.TaskID, req.ConversationID)
}

func (c *GitController) WorktreeMergeApply(ctx context.Context, req WorktreeMergeRequest, actor string) (*gitworktree.MergeAudit, error) {
	if req.PlanID == "" {
		return nil, gitworktree.ErrInvalidArgument
	}
	manager, root, err := c.checkpointManager(WorktreeCheckpointRequest{WorkspaceURI: req.WorkspaceURI, ID: req.ID, TaskID: req.TaskID, ConversationID: req.ConversationID})
	if err != nil {
		return nil, err
	}
	return manager.ApplyApprovedMerge(ctx, root, req.ID, req.TaskID, req.ConversationID, req.PlanID, actor)
}

func (c *GitController) WorktreeMergeAudit(ctx context.Context, req WorktreeMergeRequest) (*gitworktree.MergeAudit, error) {
	manager, root, err := c.checkpointManager(WorktreeCheckpointRequest{WorkspaceURI: req.WorkspaceURI, ID: req.ID, TaskID: req.TaskID, ConversationID: req.ConversationID})
	if err != nil {
		return nil, err
	}
	return manager.ReadMergeAudit(ctx, root, req.ID, req.TaskID, req.ConversationID)
}
