package desktop_pet_center

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *DesktopPetPluginManagementService
}

func NewHTTPHandler(service *DesktopPetPluginManagementService) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) RegisterRoutes(group *gin.RouterGroup) {
	plugins := group.Group("/desktop-pet/plugins")
	plugins.GET("", h.listPlugins)
	plugins.GET("/:pluginId", h.getPlugin)
	plugins.POST("/install", h.install)
	plugins.POST("/:extensionId/update", h.update)
	plugins.POST("/:extensionId/enable", h.enable)
	plugins.POST("/:extensionId/disable", h.disable)
	plugins.DELETE("/:extensionId", h.uninstall)
}

func (h *HTTPHandler) listPlugins(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	ctx := c.Request.Context()
	page := parseIntQuery(c, "page", 1)
	pageSize := parseIntQuery(c, "pageSize", 20)
	search := c.Query("search")
	resp, err := h.service.List(ctx, page, pageSize, search)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": resp})
}

func (h *HTTPHandler) getPlugin(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	ctx := c.Request.Context()
	pluginID := c.Param("pluginId")
	detail, err := h.service.Get(ctx, pluginID)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": detail})
}

func (h *HTTPHandler) install(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request body: " + err.Error(), "data": nil})
		return
	}
	result, err := h.service.Install(c.Request.Context(), req.PackagePath)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *HTTPHandler) update(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid request body: " + err.Error(), "data": nil})
		return
	}
	result, err := h.service.Update(c.Request.Context(), req.PackagePath)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *HTTPHandler) enable(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	extensionID := c.Param("extensionId")
	result, err := h.service.Enable(c.Request.Context(), extensionID)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *HTTPHandler) disable(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	extensionID := c.Param("extensionId")
	result, err := h.service.Disable(c.Request.Context(), extensionID)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *HTTPHandler) uninstall(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "desktop pet service unavailable", "data": nil})
		return
	}
	extensionID := c.Param("extensionId")
	result, err := h.service.Uninstall(c.Request.Context(), extensionID)
	if err != nil {
		writeHandlerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func writeHandlerError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 500
	switch err {
	case ErrKernelUnavailable:
		status = http.StatusServiceUnavailable
		code = 503
	case ErrExtensionNotFound:
		status = http.StatusNotFound
		code = 404
	case ErrNotDesktopPetPlugin:
		status = http.StatusForbidden
		code = 403
	case ErrInvalidInput:
		status = http.StatusBadRequest
		code = 400
	}
	c.JSON(status, gin.H{"code": code, "msg": err.Error(), "data": nil})
}

func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
	v := c.Query(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
