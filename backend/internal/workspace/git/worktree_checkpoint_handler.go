package git

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/gitworktree"
)

func authorizedCheckpointActor(c *gin.Context) (string, error) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil || actor.PrincipalType != auth.PrincipalLocalUI || !actor.IsLocalTrusted {
		return "", ErrWorktreeApprovalAuthorization
	}
	return "local_ui:" + string(actor.SpaceID) + ":" + string(actor.DeviceID), nil
}

func (h *GitHandler) handleCheckpointPrepare(c *gin.Context) {
	var req WorktreeCheckpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 30*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeCheckpointPrepare(ctx, req)
	if err != nil {
		checkpointHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleCheckpointRead(c *gin.Context) {
	var req WorktreeCheckpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 20*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeCheckpointRead(ctx, req)
	if err != nil {
		checkpointHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleCheckpointTest(c *gin.Context) {
	if _, err := authorizedCheckpointActor(c); err != nil {
		checkpointHTTPError(c, err)
		return
	}
	var req WorktreeCheckpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 100*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeCheckpointTest(ctx, req)
	if err != nil {
		checkpointHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleCheckpointApprove(c *gin.Context) {
	approver, err := authorizedCheckpointActor(c)
	if err != nil {
		checkpointHTTPError(c, err)
		return
	}
	var req WorktreeCheckpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := contextWithGitTimeout(c, 25*time.Second)
	defer cancel()
	result, err := h.controller.WorktreeCheckpointApprove(ctx, req, approver)
	if err != nil {
		checkpointHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func checkpointHTTPError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWorktreeApprovalAuthorization):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, gitworktree.ErrTestRequired), errors.Is(err, gitworktree.ErrApprovalDenied), errors.Is(err, gitworktree.ErrCheckpointRequired):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		worktreeHTTPError(c, err)
	}
}
