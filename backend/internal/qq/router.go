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
			var replyCount int64
			ctx.DB.Table("messages").Where("conversation_id IN (SELECT id FROM conversations WHERE channel = ?) AND role = ?", "qq", "user").Count(&msgCount)
			ctx.DB.Table("messages").Where("conversation_id IN (SELECT id FROM conversations WHERE channel = ?) AND role = ?", "qq", "assistant").Count(&replyCount)
			if mgr == nil {
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"data": gin.H{
						"qqOnline":     false,
						"status":       string(StatusDisconnected),
						"accountId":    "",
						"qrcodeReady":  false,
						"protocol":     "QQBot (WebSocket)",
						"startedAt":    "",
						"messageCount": msgCount,
						"replyCount":   replyCount,
					},
				})
				return
			}
			if !mgr.IsOnline() {
				online, s, accountID, err, startedAt := mgr.FetchSidecarStatus()
				if online {
					mgr.SetOnline(accountID, startedAt)
				}
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"data": gin.H{
						"qqOnline":     online,
						"status":       string(s),
						"accountId":    accountID,
						"protocol":     "QQBot (WebSocket)",
						"error":        err,
						"startedAt":    startedAt,
						"messageCount": msgCount,
						"replyCount":   replyCount,
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
					"startedAt":    mgr.GetStartedAt(),
					"messageCount": msgCount,
					"replyCount":   replyCount,
				},
			})
		})

		qqGroup.GET("/config", func(c *gin.Context) {
			mgr := GetManager()
			if mgr == nil {
				c.JSON(http.StatusOK, gin.H{"appId": "", "sandbox": false})
				return
			}
			cfg := mgr.FetchSidecarConfig()
			c.JSON(http.StatusOK, cfg)
		})
	}
}
