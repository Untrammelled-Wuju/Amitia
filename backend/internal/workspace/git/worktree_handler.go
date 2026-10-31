package git

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gitworktree"
)

func worktreeHTTPError(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, gitworktree.ErrInvalidArgument):
		code = http.StatusBadRequest
	case errors.Is(err, gitworktree.ErrNotManaged):
		code = http.StatusNotFound
	case errors.Is(err, gitworktree.ErrDirty), errors.Is(err, gitworktree.ErrBound), errors.Is(err, gitworktree.ErrBindingMismatch), errors.Is(err, gitworktree.ErrReviewOutdated):
		code = http.StatusConflict
	}
	c.JSON(code, gin.H{"error": err.Error()})
}

func (h *GitHandler) handleWorktreeList(c *gin.Context) {
	var req struct {
		WorkspaceURI string `json:"workspaceUri" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 20*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeList(ctx, req.WorkspaceURI)
	if err != nil {
		worktreeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": result})
}

func (h *GitHandler) handleWorktreeCreate(c *gin.Context) {
	var req WorktreeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.WorkspaceURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 60*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeCreate(ctx, req)
	if err != nil {
		worktreeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *GitHandler) handleWorktreeRemove(c *gin.Context) {
	var req WorktreeRemoveRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.WorkspaceURI == "" || req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 45*time.Second)
	defer cancel()
	if err := h.controller.WorktreeRemove(ctx, req); err != nil {
		worktreeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func contextWithGitTimeout(c *gin.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), timeout)
}

func (h *GitHandler) handleWorktreeRelease(c *gin.Context) {
	var req WorktreeReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.WorkspaceURI == "" || req.ID == "" || req.TaskID == "" || req.ConversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 20*time.Second)
	defer cancel()
	if err := h.controller.WorktreeRelease(ctx, req); err != nil {
		worktreeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *GitHandler) handleWorktreeReview(c *gin.Context) {
	var req WorktreeReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.WorkspaceURI == "" || req.ID == "" || req.TaskID == "" || req.ConversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 30*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeReview(ctx, req)
	if err != nil {
		worktreeHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
