package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/gamemode"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerGameModeRoutes(group *gin.RouterGroup, services *AppServices) {
	if group == nil || services == nil || services.GameMode == nil {
		return
	}
	gameMode := group.Group("/game-mode")
	gameMode.GET("", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "game mode identity is required"})
			return
		}
		if err := reconcileGameModePresence(c.Request.Context(), services, actor.SpaceID); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error(), "error": err.Error()})
			return
		}
		session, err := services.GameMode.Get(c.Request.Context(), actor.SpaceID.String())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error(), "error": err.Error()})
			return
		}
		owned := session.State != gamemode.StateInactive && session.DeviceID == actor.DeviceID.String()
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{
			"session": session, "currentDeviceId": actor.DeviceID.String(),
			"owned": owned, "canAcquire": session.State == gamemode.StateInactive || owned,
		}})
	})
	gameMode.POST("/acquire", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "game mode identity is required"})
			return
		}
		if err := reconcileGameModePresence(c.Request.Context(), services, actor.SpaceID); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error(), "error": err.Error()})
			return
		}
		if services.DeviceMesh != nil && services.DeviceMesh.Coordination != nil {
			if err := services.DeviceMesh.Coordination.RequireCapabilityExposure(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), "game.execute"); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error(), "error": err.Error()})
				return
			}
		}
		var request struct {
			GameExtensionID string `json:"gameExtensionId"`
			GamePluginID    string `json:"gamePluginId"`
			GameTitle       string `json:"gameTitle"`
			RuntimeID       string `json:"runtimeId"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "error": err.Error()})
			return
		}
		session, err := services.GameMode.Acquire(c.Request.Context(), gamemode.AcquireInput{
			SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(),
			GameExtensionID: request.GameExtensionID, GamePluginID: request.GamePluginID,
			GameTitle: request.GameTitle, RuntimeID: request.RuntimeID,
		})
		if err != nil {
			if errors.Is(err, gamemode.ErrOccupied) {
				current, readErr := services.GameMode.Get(c.Request.Context(), actor.SpaceID.String())
				if readErr != nil {
					c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error(), "error": err.Error()})
					return
				}
				c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error(), "error": err.Error(), "data": current})
				return
			}
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error(), "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": session})
	})
	gameMode.POST("/update", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "game mode identity is required"})
			return
		}
		var request struct {
			SessionID        string `json:"sessionId"`
			State            string `json:"state"`
			GameTitle        string `json:"gameTitle"`
			RuntimeID        string `json:"runtimeId"`
			ExpectedRevision int64  `json:"expectedRevision"`
		}
		if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.SessionID) == "" {
			message := "invalid game mode update"
			if err != nil {
				message = err.Error()
			}
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": message, "error": message})
			return
		}
		session, err := services.GameMode.Update(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), request.SessionID, gamemode.UpdateInput{
			State: request.State, GameTitle: request.GameTitle, RuntimeID: request.RuntimeID, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error(), "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": session})
	})
	gameMode.POST("/release", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "game mode identity is required"})
			return
		}
		var request struct {
			SessionID        string `json:"sessionId"`
			ExpectedRevision int64  `json:"expectedRevision"`
			Force            bool   `json:"force"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "error": err.Error()})
			return
		}
		force := request.Force && actor.HasPermission(auth.PermSystemAdmin)
		session, err := services.GameMode.Release(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), request.SessionID, request.ExpectedRevision, force)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error(), "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": session})
	})
}
