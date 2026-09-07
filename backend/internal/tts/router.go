// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"github.com/gin-gonic/gin"
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
		ttsGroup.POST("/synthesize", handler.Synthesize)
		ttsGroup.POST("/preview", handler.Preview)
		ttsGroup.POST("/test-connection", security.SharedCoreAdminOnly(), handler.TestConnectionStandalone)
		ttsGroup.GET("/voice-clones", handler.ListClonedVoices)
		ttsGroup.POST("/voice-clone", handler.CloneVoice)
		ttsGroup.DELETE("/voice-clone", handler.DeleteClonedVoice)
		ttsGroup.GET("/play/:messageId", func(c *gin.Context) { HandlePlayMessage(c, ctx.DB) })
	}
}
