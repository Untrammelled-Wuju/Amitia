package workspace

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/workspaces")
	{
		group.GET("", h.listMounts)
		group.GET("/:id", h.getMount)
		group.DELETE("/:id", h.removeMount)
		group.POST("/local", h.registerLocalMount)
		group.POST("/:id/touch", h.touchMount)
		group.POST("/saf", h.registerSAFMount)
		group.PUT("/:id/grant", h.replaceSAFGrant)
		group.POST("/:id/refresh", h.refreshMountStatus)
		group.POST("/remote", h.registerRemoteMount)
		group.PATCH("/:id/remote", h.updateRemoteMount)
		group.GET("/stat", h.statResource)
		group.GET("/list", h.listResources)
		group.GET("/read", h.readResource)
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

func (h *Handler) registerLocalMount(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		LocalRoot string `json:"localRoot" binding:"required"`
		ReadOnly  bool   `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mount, err := h.service.RegisterLocalMount(c.Request.Context(), req.Name, req.LocalRoot, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mount)
}

func (h *Handler) touchMount(c *gin.Context) {
	mount, err := h.service.TouchMount(c.Request.Context(), WorkspaceID(strings.TrimSpace(c.Param("id"))))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mount)
}

func (h *Handler) statResource(c *gin.Context) {
	uri := strings.TrimSpace(c.Query("uri"))
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}
	entry, err := h.service.Stat(c.Request.Context(), uri)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (h *Handler) listResources(c *gin.Context) {
	uri := strings.TrimSpace(c.Query("uri"))
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	result, err := h.service.List(c.Request.Context(), uri, ListOptions{Limit: limit, Cursor: c.Query("cursor")})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) readResource(c *gin.Context) {
	uri := strings.TrimSpace(c.Query("uri"))
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	maxBytes, _ := strconv.ParseInt(c.DefaultQuery("maxBytes", "1048576"), 10, 64)
	result, err := h.service.Read(c.Request.Context(), uri, ReadOptions{Offset: offset, MaxBytes: maxBytes, Encoding: c.DefaultQuery("encoding", "utf-8")})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"resource": result.Resource, "isText": result.IsText, "content": string(result.Content)})
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

func (h *Handler) registerRemoteMount(c *gin.Context) {
	var req struct {
		Name          string            `json:"name" binding:"required"`
		Config        RemoteMountConfig `json:"config" binding:"required"`
		CredentialRef string            `json:"credentialRef"`
		ReadOnly      bool              `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mount, err := h.service.RegisterRemoteMount(c.Request.Context(), req.Name, req.Config, req.CredentialRef, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mount)
}

func (h *Handler) updateRemoteMount(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Config        RemoteMountConfig `json:"config" binding:"required"`
		CredentialRef string            `json:"credentialRef"`
		ReadOnly      bool              `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mount, err := h.service.UpdateRemoteMountConfig(c.Request.Context(), WorkspaceID(id), req.Config, req.CredentialRef, req.ReadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mount)
}
