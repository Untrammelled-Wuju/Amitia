package git

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gitworktree"
	"net/http"
	"time"
)

func (h *GitHandler) handleWorktreeMergePreview(c *gin.Context) {
	var req WorktreeMergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 20*time.Second)
	defer cancel()
	plan, err := h.controller.WorktreeMergePreview(ctx, req)
	if err != nil {
		mergeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *GitHandler) handleWorktreeMergeApply(c *gin.Context) {
	actor, err := authorizedCheckpointActor(c)
	if err != nil {
		mergeHTTPError(c, err)
		return
	}
	var req WorktreeMergeRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 45*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeMergeApply(ctx, req, actor)
	if err != nil {
		mergeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleWorktreeMergeAudit(c *gin.Context) {
	var req WorktreeMergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 20*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeMergeAudit(ctx, req)
	if err != nil {
		mergeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func mergeHTTPError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gitworktree.ErrMergeNotFastForward), errors.Is(err, gitworktree.ErrMergeTargetChanged), errors.Is(err, gitworktree.ErrMergePreviouslyAttempted):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		checkpointHTTPError(c, err)
	}
}
