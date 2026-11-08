package management

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

type GameBehaviorHandler struct {
	service *GameBehaviorService
}

func NewGameBehaviorHandler(service *GameBehaviorService) *GameBehaviorHandler {
	return &GameBehaviorHandler{service: service}
}

func (h *GameBehaviorHandler) List(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "game behavior service unavailable", "message": "game behavior service unavailable", "error": "game behavior service unavailable"})
		return
	}
	pluginID := strings.TrimSpace(c.Query("pluginId"))
	if pluginID == "" {
		pluginID = strings.TrimSpace(c.Param("pluginId"))
	}
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "pluginId required", "message": "pluginId required"})
		return
	}
	result, err := h.service.List(c.Request.Context(), pluginID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error(), "message": err.Error(), "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}

func (h *GameBehaviorHandler) Invoke(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "game behavior service unavailable", "message": "game behavior service unavailable", "error": "game behavior service unavailable"})
		return
	}
	actor := security.GetActor(c)
	if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "game behavior identity is required", "message": "game behavior identity is required", "error": "game behavior identity is required"})
		return
	}
	var request struct {
		PluginID                string          `json:"pluginId"`
		BehaviorID              string          `json:"behaviorId"`
		RuntimeID               string          `json:"runtimeId"`
		Arguments               json.RawMessage `json:"arguments"`
		ExpectedCatalogRevision string          `json:"expectedCatalogRevision"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "message": err.Error(), "error": err.Error()})
		return
	}
	pluginID := strings.TrimSpace(request.PluginID)
	behaviorID := strings.TrimSpace(request.BehaviorID)
	if pluginID == "" || behaviorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "pluginId and behaviorId are required", "message": "pluginId and behaviorId are required", "error": "pluginId and behaviorId are required"})
		return
	}
	if err := h.service.Authorize(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), pluginID, behaviorID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error(), "message": err.Error(), "error": err.Error()})
		return
	}
	result, err := h.service.Invoke(c.Request.Context(), pluginID, behaviorID, request.RuntimeID, request.ExpectedCatalogRevision, request.Arguments)
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "catalog revision changed") || strings.Contains(err.Error(), "behavior not found") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"code": status, "msg": err.Error(), "message": err.Error(), "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
}
