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
	if err := h.service.registry.RemoveMount(c.Request.Context(), WorkspaceID(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
