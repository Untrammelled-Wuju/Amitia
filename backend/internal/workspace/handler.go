package workspace

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/api/workspaces")
	{
		group.GET("", h.listMounts)
		group.GET("/:id", h.getMount)
		group.DELETE("/:id", h.removeMount)
		group.POST("/saf", h.registerSAFMount)
		group.PUT("/:id/grant", h.replaceSAFGrant)
		group.POST("/:id/refresh", h.refreshMountStatus)
	}
}

func (h *Handler) listMounts(c *gin.Context) {
	mounts, err := h.service.ListMounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mounts)
}

func (h *Handler) getMount(c *gin.Context) {
	id := c.Param("id")
	mount, ok := h.service.registry.GetMount(WorkspaceID(id))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, mount)
}

func (h *Handler) removeMount(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RemoveMount(c.Request.Context(), WorkspaceID(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *Handler) registerSAFMount(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		GrantID  string `json:"grantId" binding:"required"`
		ReadOnly bool   `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mount, err := h.service.RegisterSAFMount(c.Request.Context(), req.Name, req.GrantID, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mount)
}

func (h *Handler) replaceSAFGrant(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		GrantID  string `json:"grantId" binding:"required"`
		ReadOnly bool   `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mount, err := h.service.ReplaceSAFGrant(c.Request.Context(), WorkspaceID(id), req.GrantID, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mount)
}

func (h *Handler) refreshMountStatus(c *gin.Context) {
	id := c.Param("id")
	mount, err := h.service.RefreshMountStatus(c.Request.Context(), WorkspaceID(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mount)
}
