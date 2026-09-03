// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterRealtimeRouter(r *gin.RouterGroup, ctx *app.AppContext, visionSvc vision.Service) {
	SetDB(ctx.DB)
	SetVisualAnalyzer(NewConfiguredVisualAnalyzer(visionSvc))

	// Legacy route remains available while clients migrate. It now resolves
	// provider credentials on the server and supports the v2 call metadata.
	r.GET("/realtime/session", HandleSession)
	r.POST("/realtime/v2/tickets", IssueRealtimeAccessTicket)
	r.GET("/realtime/v2/session", HandleSession)
	r.GET("/realtime/v2/visual", HandleVisualSession)

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
