package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	artifact "github.com/u-ai/backend/internal/gamehost/companion"
)

// ArtifactHandler exposes generic deployment operations for artifacts declared
// by a game plugin. It deliberately has no knowledge of game installation
// layouts, loaders, worlds or versions. Callers provide an explicitly approved
// target root and an opaque compatibility version understood by the plugin.
type ArtifactHandler struct{ manager *artifact.ArtifactManager }

func NewArtifactHandler(manager *artifact.ArtifactManager) *ArtifactHandler {
	return &ArtifactHandler{manager: manager}
}

type artifactDeploymentRequest struct {
	TargetRoot           string `json:"targetRoot"`
	CompatibilityVersion string `json:"compatibilityVersion,omitempty"`
}

func (h *ArtifactHandler) List(c *gin.Context) {
	if h == nil || h.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "artifact manager unavailable"})
		return
	}
	root := strings.TrimSpace(c.Query("targetRoot"))
	version := strings.TrimSpace(c.Query("compatibilityVersion"))
	items, err := h.manager.List(c.Request.Context(), c.Param("extensionId"), root, version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ArtifactHandler) Deploy(c *gin.Context) {
	if h == nil || h.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "artifact manager unavailable"})
		return
	}
	var req artifactDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.manager.Deploy(c.Request.Context(), c.Param("extensionId"), c.Param("artifactId"), req.TargetRoot, req.CompatibilityVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *ArtifactHandler) DeployRequired(c *gin.Context) {
	if h == nil || h.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "artifact manager unavailable"})
		return
	}
	var req artifactDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.manager.DeployRequiredArtifacts(c.Request.Context(), c.Param("extensionId"), req.TargetRoot, req.CompatibilityVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ArtifactHandler) Verify(c *gin.Context) {
	if h == nil || h.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "artifact manager unavailable"})
		return
	}
	var req artifactDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.manager.Verify(c.Request.Context(), c.Param("extensionId"), c.Param("artifactId"), req.TargetRoot, req.CompatibilityVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *ArtifactHandler) Remove(c *gin.Context) {
	if h == nil || h.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "artifact manager unavailable"})
		return
	}
	var req artifactDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Remove(c.Request.Context(), c.Param("extensionId"), c.Param("artifactId"), req.TargetRoot); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
