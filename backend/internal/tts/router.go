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
		ttsGroup.POST("/configs/:id/preview", security.SharedCoreAdminOnly(), handler.PreviewConfig)
		ttsGroup.GET("/voices", handler.GetVoices)
		ttsGroup.GET("/emotions", handler.GetEmotions)
		ttsGroup.POST("/preview", security.SharedCoreAdminOnly(), handler.Preview)
		ttsGroup.POST("/characters/:characterId/preview", security.SharedCoreAdminOnly(), handler.PreviewCharacter)
		ttsGroup.POST("/test-connection", security.SharedCoreAdminOnly(), handler.TestConnectionStandalone)
		ttsGroup.GET("/voice-clones", handler.ListClonedVoices)
		ttsGroup.POST("/voice-clone", security.SharedCoreAdminOnly(), handler.CloneVoice)
		ttsGroup.DELETE("/voice-clone", security.SharedCoreAdminOnly(), handler.DeleteClonedVoice)
		ttsGroup.POST("/voice-clones/:speakerId/preview", security.SharedCoreAdminOnly(), handler.PreviewClonedVoice)
	}
}
