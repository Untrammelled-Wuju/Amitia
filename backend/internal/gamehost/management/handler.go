package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *GameCenterManagementService
}

func NewHandler(service *GameCenterManagementService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListPlugins(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	var filter PluginFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid filter parameters"})
		return
	}
	filter = filter.Normalize()

	result, err := h.service.ListPlugins(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) GetPlugin(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	pluginID := strings.TrimSpace(c.Param("pluginId"))
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "pluginId required"})
		return
	}

	extensionID := strings.TrimSpace(c.Query("extensionId"))
	if extensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "extensionId required"})
		return
	}

	result, err := h.service.GetPlugin(c.Request.Context(), extensionID, pluginID)
	if err != nil {
		if err.Error() == "extension not found" || err.Error() == "plugin not found" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) ListRuntimes(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	var filter RuntimeFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid filter parameters"})
		return
	}
	filter = filter.Normalize()

	result, err := h.service.ListRuntimes(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) GetRuntime(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	pluginID := strings.TrimSpace(c.Query("pluginId"))
	result, err := h.service.GetRuntime(c.Request.Context(), runtimeID, pluginID)
	if err != nil {
		if err.Error() == "runtime not found" || err.Error() == "runtime does not belong to specified plugin" || err.Error() == "runtime does not belong to game center" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) ListServices(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.service.ListServices(c.Request.Context(), runtimeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) GetRuntimeHealth(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Param("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.service.GetRuntimeHealth(c.Request.Context(), runtimeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) GetPluginHealth(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	pluginID := strings.TrimSpace(c.Param("pluginId"))
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "pluginId required"})
		return
	}

	health := h.service.pluginHealth(pluginID)
	result := HealthSummaryDTO{Status: health}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) GetHandshakeStatus(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	connectionID := strings.TrimSpace(c.Query("connectionId"))
	runtimeID := strings.TrimSpace(c.Query("runtimeId"))

	target := ""
	if runtimeID != "" {
		target = runtimeID
	} else if connectionID != "" {
		target = connectionID
	}

	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId or connectionId required"})
		return
	}

	result, err := h.service.GetHandshakeStatus(c.Request.Context(), target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *Handler) GetControlAuthority(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "service unavailable"})
		return
	}

	runtimeID := strings.TrimSpace(c.Query("runtimeId"))
	if runtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId required"})
		return
	}

	result, err := h.service.GetControlAuthority(c.Request.Context(), runtimeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}
