package git

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GitHandler struct {
	controller *GitController
}

func NewGitHandler(controller *GitController) *GitHandler {
	return &GitHandler{controller: controller}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	result, err := h.controller.CreateIsolatedFromClone(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
