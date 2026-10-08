// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterTtsRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB)
	svc := NewService(repo)
	handler := NewHandler(svc)

	ttsGroup := r.Group("/tts")
	{
		ttsGroup.GET("/providers", handler.ListProviders)
		ttsGroup.GET("/configs", security.SharedCoreAdminOnly(), handler.List)
		ttsGroup.GET("/config-summaries", handler.ListSummaries)
		ttsGroup.GET("/configs/:id", security.SharedCoreAdminOnly(), handler.Get)
		ttsGroup.POST("/configs", security.SharedCoreAdminOnly(), handler.Create)
		ttsGroup.PUT("/configs/:id", security.SharedCoreAdminOnly(), handler.Update)
		ttsGroup.DELETE("/configs/:id", security.SharedCoreAdminOnly(), handler.Delete)
		ttsGroup.POST("/configs/:id/activate", security.SharedCoreAdminOnly(), handler.Activate)
		ttsGroup.POST("/configs/:id/test", security.SharedCoreAdminOnly(), handler.Test)
		ttsGroup.GET("/voices", handler.GetVoices)
		ttsGroup.GET("/emotions", handler.GetEmotions)
		ttsGroup.POST("/synthesize", ownedSpeechRequired(), handler.Synthesize)
		ttsGroup.POST("/preview", ownedSpeechRequired(), handler.Preview)
		ttsGroup.POST("/test-connection", security.SharedCoreAdminOnly(), handler.TestConnectionStandalone)
		ttsGroup.GET("/voice-clones", handler.ListClonedVoices)
		ttsGroup.POST("/voice-clone", security.SharedCoreAdminOnly(), handler.CloneVoice)
		ttsGroup.DELETE("/voice-clone", security.SharedCoreAdminOnly(), handler.DeleteClonedVoice)
		ttsGroup.GET("/play/:messageId", ownedSpeechRequired(), func(c *gin.Context) { HandlePlayMessage(c, ctx.DB) })
	}
}

func ownedSpeechRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor != nil && actor.PrincipalType == auth.PrincipalTrustedDevice {
			c.AbortWithStatusJSON(409, gin.H{"code": "mesh.speech_scope_required", "message": "绑定设备朗读需要当前角色和数据归属版本，请使用统筹语音入口"})
			return
		}
		c.Next()
	}
}
