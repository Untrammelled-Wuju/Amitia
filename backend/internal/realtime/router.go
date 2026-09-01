// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterRealtimeRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	SetDB(ctx.DB)
	r.GET("/realtime/session", HandleSession)

	handler := NewVoiceHandler(NewService())
	voiceGroup := r.Group("/voice")
	voiceGroup.POST("/sessions", handler.CreateSession)
	voiceGroup.GET("/sessions", handler.ListSessions)
	voiceGroup.GET("/sessions/:id", handler.GetSession)
	voiceGroup.POST("/sessions/:id/start", handler.StartSession)
	voiceGroup.POST("/sessions/:id/stop", handler.StopSession)
	voiceGroup.POST("/sessions/:id/interrupt", handler.InterruptSession)
	voiceGroup.POST("/sessions/:id/wake/arm", handler.ArmWake)
	voiceGroup.POST("/sessions/:id/wake/disarm", handler.DisarmWake)
	voiceGroup.POST("/sessions/:id/asr/final", handler.PublishASRFinal)
	voiceGroup.GET("/status", handler.GetStatus)
}
