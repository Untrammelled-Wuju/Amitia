// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
)

func RegisterInternalRoutes(r *gin.Engine, svc *Service) {
	r.GET(svc.Config().Path, gin.WrapH(svc.Handler()))
}

func RegisterUserRoutes(apiGroup *gin.RouterGroup, svc *Service) {
	runtimeGroup := apiGroup.Group("/desktop-pets/runtime")

	runtimeGroup.GET("/status", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		statuses, err := svc.ListRuntimeStatuses(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "INTERNAL", "message": "查询失败"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": statuses})
	})

	runtimeGroup.GET("/status/:runtimeId", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		runtimeID := c.Param("runtimeId")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		status, err := svc.GetRuntimeStatus(c.Request.Context(), userID, runtimeID)
		if err != nil {
			if re, ok := err.(*RuntimeError); ok {
				c.JSON(MapRuntimeErrorCodeToHTTP(re.Code), gin.H{"success": false, "error": gin.H{"code": re.Code, "message": re.Message}})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "INTERNAL", "message": "查询失败"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
	})

	runtimeGroup.GET("/metrics", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		actor, err := middleware.GetActorFromContext(c)
		if err != nil || !actor.HasRole("admin") {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "FORBIDDEN", "message": "需要管理员权限"}})
			return
		}
		snapshot := svc.GetMetrics()
		c.JSON(http.StatusOK, gin.H{"success": true, "data": snapshot})
	})

	runtimeGroup.GET("/bootstrap-token", func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "认证失败"}})
			return
		}
		info := svc.auth.BootstrapTokenInfo(svc.Config().Path)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": info})
	})
}

func WriteJSON(c *gin.Context, status int, data interface{}) {
	c.Header("Content-Type", "application/json")
	c.Status(status)
	json.NewEncoder(c.Writer).Encode(data)
}
