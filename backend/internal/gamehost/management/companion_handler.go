package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/gamehost/companion"
)

type CompanionHandler struct{ manager *companion.Manager }

func NewCompanionHandler(manager *companion.Manager) *CompanionHandler {
	return &CompanionHandler{manager: manager}
}

type companionRequest struct {
	GameRoot    string `json:"gameRoot"`
	GameVersion string `json:"gameVersion,omitempty"`
}

func (h *CompanionHandler) List(c *gin.Context) {
	if h == nil || h.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "companion manager unavailable"})
		return
	}
	root := strings.TrimSpace(c.Query("gameRoot"))
	version := strings.TrimSpace(c.Query("gameVersion"))
	items, err := h.manager.List(c.Request.Context(), c.Param("extensionId"), root, version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *CompanionHandler) Install(c *gin.Context) {
	var req companionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.manager.Install(c.Request.Context(), c.Param("extensionId"), c.Param("artifactId"), req.GameRoot, req.GameVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CompanionHandler) InstallRequired(c *gin.Context) {
	var req companionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.manager.InstallRequired(c.Request.Context(), c.Param("extensionId"), req.GameRoot, req.GameVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *CompanionHandler) Verify(c *gin.Context) {
	var req companionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.manager.Verify(c.Request.Context(), c.Param("extensionId"), c.Param("artifactId"), req.GameRoot, req.GameVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CompanionHandler) Remove(c *gin.Context) {
	var req companionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Remove(c.Request.Context(), c.Param("extensionId"), c.Param("artifactId"), req.GameRoot); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
