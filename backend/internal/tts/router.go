// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterTtsRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB)
	svc := NewService(repo)
	handler := NewHandler(svc)

	ttsGroup := r.Group("/tts")
	{
		ttsGroup.GET("/configs", handler.List)
		ttsGroup.GET("/configs/:id", handler.Get)
		ttsGroup.POST("/configs", handler.Create)
		ttsGroup.PUT("/configs/:id", handler.Update)
		ttsGroup.DELETE("/configs/:id", handler.Delete)
		ttsGroup.POST("/configs/:id/activate", handler.Activate)
		ttsGroup.POST("/configs/:id/test", handler.Test)
		ttsGroup.GET("/voices", handler.GetVoices)
		ttsGroup.GET("/emotions", handler.GetEmotions)
		ttsGroup.POST("/synthesize", handler.Synthesize)
		ttsGroup.POST("/voice-clone", handler.CloneVoice)
		ttsGroup.DELETE("/voice-clone", handler.DeleteClonedVoice)
		ttsGroup.GET("/play/:messageId", func(c *gin.Context) { HandlePlayMessage(c, ctx.DB) })
	}
}
