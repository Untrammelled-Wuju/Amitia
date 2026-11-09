package git

import (
	"context"
	"encoding/json"
	"github.com/u-ai/backend/internal/gitworktree"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GitHandler struct {
	controller *GitController
}

func NewGitHandler(controller *GitController) *GitHandler {
	return &GitHandler{controller: controller}
}

func (h *GitHandler) Status(ctx context.Context, workspaceURI string, includeIgnored bool) (*GitStatusResult, error) {
	return h.controller.Status(ctx, workspaceURI, includeIgnored)
}

func (h *GitHandler) Diff(ctx context.Context, req GitDiffRequest) (*GitDiffResult, error) {
	return h.controller.Diff(ctx, req)
}

func (h *GitHandler) Log(ctx context.Context, req GitLogRequest) (*GitLogResult, error) {
	return h.controller.Log(ctx, req)
}

func (h *GitHandler) Add(ctx context.Context, req GitAddRequest) (*GitAddResult, error) {
	return h.controller.Add(ctx, req)
}

func (h *GitHandler) Restore(ctx context.Context, req GitRestoreRequest) (*GitRestoreResult, error) {
	return h.controller.Restore(ctx, req)
}

func (h *GitHandler) Commit(ctx context.Context, req GitCommitRequest) (*GitCommitResult, error) {
	return h.controller.Commit(ctx, req)
}

func (h *GitHandler) ListBranches(ctx context.Context, workspaceURI string) (*GitBranchListResult, error) {
	return h.controller.ListBranches(ctx, workspaceURI)
}

func (h *GitHandler) Checkout(ctx context.Context, req GitCheckoutRequest) (*GitCheckoutResult, error) {
	return h.controller.Checkout(ctx, req)
}

func (h *GitHandler) Fetch(ctx context.Context, req GitFetchRequest) (*GitFetchResult, error) {
	return h.controller.Fetch(ctx, req)
}

func (h *GitHandler) Pull(ctx context.Context, req GitPullRequest) (*GitPullResult, error) {
	return h.controller.Pull(ctx, req)
}

func (h *GitHandler) Push(ctx context.Context, req GitPushRequest) (*GitPushResult, error) {
	return h.controller.Push(ctx, req)
}

func (h *GitHandler) ListRemotes(ctx context.Context, workspaceURI string) ([]GitRemoteInfo, error) {
	return h.controller.ListRemotes(ctx, workspaceURI)
}

func (h *GitHandler) CreateIsolated(ctx context.Context, req IsolatedCreateRequest) (*IsolatedCreateResult, error) {
	return h.controller.CreateIsolated(ctx, req)
}

func (h *GitHandler) DeleteIsolated(ctx context.Context, workspaceURI string, force bool) error {
	return h.controller.DeleteIsolated(ctx, workspaceURI, force)
}

func (h *GitHandler) IsolatedInfo(ctx context.Context, workspaceURI string) (*IsolatedWorkspaceInfo, error) {
	return h.controller.IsolatedInfo(ctx, workspaceURI)
}

func (h *GitHandler) CreateTaskWorktree(ctx context.Context, req WorktreeCreateRequest) (*gitworktree.Entry, error) {
	if h == nil || h.controller == nil || req.TaskID == "" || req.ConversationID == "" {
		return nil, gitworktree.ErrInvalidArgument
	}
	return h.controller.WorktreeCreate(ctx, req)
}

func (h *GitHandler) ListTaskWorktrees(ctx context.Context, workspaceURI string) ([]gitworktree.Entry, error) {
	if h == nil || h.controller == nil || workspaceURI == "" {
		return nil, gitworktree.ErrInvalidArgument
	}
	return h.controller.WorktreeList(ctx, workspaceURI)
}

func (h *GitHandler) ReviewTaskWorktree(ctx context.Context, req WorktreeReviewRequest) (*WorktreeReviewResult, error) {
	if h == nil || h.controller == nil {
		return nil, gitworktree.ErrInvalidArgument
	}
	return h.controller.WorktreeReview(ctx, req)
}

func (h *GitHandler) ReadTaskCheckpoint(ctx context.Context, req WorktreeCheckpointRequest) (*gitworktree.ReviewCheckpoint, error) {
	if h == nil || h.controller == nil {
		return nil, gitworktree.ErrInvalidArgument
	}
	return h.controller.WorktreeCheckpointRead(ctx, req)
}

func (h *GitHandler) ReadTaskMergeAudit(ctx context.Context, req WorktreeMergeRequest) (*gitworktree.MergeAudit, error) {
	if h == nil || h.controller == nil {
		return nil, gitworktree.ErrInvalidArgument
	}
	return h.controller.WorktreeMergeAudit(ctx, req)
}

func (h *GitHandler) RegisterRoutes(r gin.IRouter) {
	git := r.Group("/workspaces/git")
	{
		git.POST("/status", h.handleStatus)
		git.POST("/diff", h.handleDiff)
		git.POST("/log", h.handleLog)
		git.POST("/add", h.handleAdd)
		git.POST("/restore", h.handleRestore)
		git.POST("/commit", h.handleCommit)
		git.POST("/branches", h.handleListBranches)
		git.POST("/checkout", h.handleCheckout)
		git.POST("/fetch", h.handleFetch)
		git.POST("/pull", h.handlePull)
		git.POST("/push", h.handlePush)
		git.POST("/remotes", h.handleListRemotes)
		git.POST("/worktrees", h.handleWorktreeList)
		git.POST("/worktrees/create", h.handleWorktreeCreate)
		git.POST("/worktrees/remove", h.handleWorktreeRemove)
		git.POST("/worktrees/release", h.handleWorktreeRelease)
		git.POST("/worktrees/review", h.handleWorktreeReview)
		git.POST("/worktrees/checkpoints/prepare", h.handleCheckpointPrepare)
		git.POST("/worktrees/checkpoints/read", h.handleCheckpointRead)
		git.POST("/worktrees/checkpoints/test", h.handleCheckpointTest)
		git.POST("/worktrees/checkpoints/approve", h.handleCheckpointApprove)
		git.POST("/worktrees/merges/preview", h.handleWorktreeMergePreview)
		git.POST("/worktrees/merges/apply", h.handleWorktreeMergeApply)
		git.POST("/worktrees/merges/audit", h.handleWorktreeMergeAudit)
	}
	isolated := r.Group("/workspaces/isolated")
	{
		isolated.POST("", h.handleCreateIsolated)
		isolated.POST("/delete", h.handleDeleteIsolated)
		isolated.POST("/info", h.handleIsolatedInfo)
	}
}

func (h *GitHandler) handleStatus(c *gin.Context) {
	var req struct {
		WorkspaceURI   string `json:"workspaceUri" binding:"required"`
		IncludeIgnored bool   `json:"includeIgnored"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Status(c.Request.Context(), req.WorkspaceURI, req.IncludeIgnored)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleDiff(c *gin.Context) {
	var req GitDiffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Diff(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleLog(c *gin.Context) {
	var req GitLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Log(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleAdd(c *gin.Context) {
	var req GitAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Add(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleRestore(c *gin.Context) {
	var req GitRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Restore(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleCommit(c *gin.Context) {
	var req GitCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Commit(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleListBranches(c *gin.Context) {
	var req struct {
		WorkspaceURI string `json:"workspaceUri" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.ListBranches(c.Request.Context(), req.WorkspaceURI)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleCheckout(c *gin.Context) {
	var req GitCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Checkout(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleFetch(c *gin.Context) {
	var req GitFetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Fetch(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handlePull(c *gin.Context) {
	var req GitPullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Pull(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handlePush(c *gin.Context) {
	var req GitPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.Push(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleListRemotes(c *gin.Context) {
	var req struct {
		WorkspaceURI string `json:"workspaceUri" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.ListRemotes(c.Request.Context(), req.WorkspaceURI)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GitHandler) handleCreateIsolated(c *gin.Context) {
	var req IsolatedCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.CreateIsolated(c.Request.Context(), req)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *GitHandler) handleDeleteIsolated(c *gin.Context) {
	var req struct {
		WorkspaceURI string `json:"workspaceUri" binding:"required"`
		Force        bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.controller.DeleteIsolated(c.Request.Context(), req.WorkspaceURI, req.Force); err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *GitHandler) handleIsolatedInfo(c *gin.Context) {
	var req struct {
		WorkspaceURI string `json:"workspaceUri" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.controller.IsolatedInfo(c.Request.Context(), req.WorkspaceURI)
	if err != nil {
		WriteHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func writeJSONError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}

func jsonData(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
