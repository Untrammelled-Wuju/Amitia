// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qq

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

var defaultManager *Manager

func GetManager() *Manager    { return defaultManager }
func SetManager(mgr *Manager) { defaultManager = mgr }

func RegisterQQRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	qqGroup := r.Group("/qq")
	{
		qqGroup.POST("/connect", func(c *gin.Context) {
			var req struct {
				AppID   string `json:"appId"`
				Token   string `json:"token"`
				Sandbox bool   `json:"sandbox"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "appId and token required"})
				return
			}
			mgr := GetManager()
			if mgr == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "manager not initialized"})
				return
			}
			go func() { _ = mgr.Connect(req.AppID, req.Token, req.Sandbox) }()
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		qqGroup.POST("/disconnect", func(c *gin.Context) {
			mgr := GetManager()
			if mgr != nil {
				mgr.Disconnect()
			}
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		qqGroup.GET("/status", func(c *gin.Context) {
			mgr := GetManager()
			var msgCount int64
			ctx.DB.Table("messages").Joins("JOIN conversations ON messages.conversation_id = conversations.id").Where("conversations.channel = ?", "qq").Count(&msgCount)
			if mgr == nil {
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"data": gin.H{
						"qqOnline":     false,
						"status":       string(StatusDisconnected),
						"accountId":    "",
						"qrcodeReady":  false,
						"protocol":     "QQBot (WebSocket)",
						"messageCount": msgCount,
					},
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"qqOnline":     mgr.IsOnline(),
					"status":       string(mgr.GetStatus()),
					"accountId":    mgr.GetAccountID(),
					"protocol":     "QQBot (WebSocket)",
					"error":        mgr.GetLastError(),
					"messageCount": msgCount,
				},
			})
		})

		qqGroup.GET("/config", func(c *gin.Context) {
			mgr := GetManager()
			if mgr == nil {
				c.JSON(http.StatusOK, gin.H{"appId": "", "sandbox": false})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"appId":   mgr.GetAppID(),
				"sandbox": mgr.GetSandbox(),
			})
		})
	}
}
