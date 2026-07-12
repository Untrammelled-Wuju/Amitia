// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent"
	"github.com/u-ai/backend/internal/aicharacter"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/feedback"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qq"
	"github.com/u-ai/backend/internal/realtime"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/internal/user"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/pkg/app"
)

func setupRouter(ctx *app.AppContext, services *AppServices) *gin.Engine {
	if config.AppCfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(middleware.TraceMiddleware())
	r.Use(security.CorsMiddleware())
	apiGroup := r.Group("/api")
	{
		user.RegisterUserRouter(apiGroup, ctx)
		character.RegisterCharacterRouter(apiGroup, ctx, services.Chat)
		chat.RegisterChatRouterWithDelivery(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.ChatDeliveryAdapter)
		memHandler := memory.RegisterMemoryRouter(apiGroup, ctx, services.Graph)
		apiGroup.GET("/memory/retrieval/stats", memHandler.RetrieveStats)
		apiGroup.GET("/memory/pipeline/status", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "data": services.Chat.GetPipelineStatus(), "msg": "\u64cd\u4f5c\u6210\u529f"})
		})
		profile.RegisterProfileRouter(apiGroup, services.Profile)
		proHandler := proactive.RegisterProactiveRouterWithCompanion(apiGroup, ctx, services.Companion)
		proactive.RegisterRemindersRouter(apiGroup, proHandler)
		episodic.RegisterEpisodicRouter(apiGroup, services.Episodic)
		worldbook.RegisterWorldBookRouter(apiGroup, services.WorldBook)
		feedback.RegisterFeedbackRouter(apiGroup, ctx)
		graph.RegisterGraphRouter(apiGroup, config.AppCfg.Surreal)
		agent.RegisterAgentRouter(apiGroup, ctx, services.UnifiedEntry)
		aicharacter.RegisterAICharacterRouter(apiGroup, ctx)
		system.RegisterSystemRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.DataLifecycle, services.Memory, services.Profile, services.Episodic, services.Graph)
		companion.RegisterCompanionRouter(apiGroup, services.Companion)
		qq.RegisterQQRouter(apiGroup, ctx)
		tts.RegisterTtsRouter(apiGroup, ctx)
		asr.RegisterAsrRouter(apiGroup, ctx)
		realtime.RegisterRealtimeRouter(apiGroup, ctx)
		vision.RegisterVisionRouter(apiGroup, ctx)
		embedding_config.RegisterEmbeddingConfigRouter(apiGroup, ctx)
		system.RegisterPsycheAPIRouter(apiGroup)
		system.RegisterPsycheSnapshotRouter(apiGroup, ctx.DB)
		system.RegisterHealthRouter(apiGroup, services.CircuitBreakers, services.DataLifecycle, services.Reconciliation)
		ttsRepo := tts.NewRepository(ctx.DB)
		ttsSvc := tts.NewService(ttsRepo)
		system.RegisterVoiceEntryRouter(apiGroup, services.VoiceEntry, ttsSvc, services.DeliveryStore)
		safety.RegisterSafetyRouter(apiGroup, ctx.DB)
		delivery.RegisterSubmitRouter(apiGroup, services.DeliveryStore)
	}
	r.Static("/audio", "./data/tts_cache")
	r.Static("/voice", "./data/voice_msg")
	r.Static("/images", "./data/images")
	r.Static("/videos", "./data/videos")
	r.Static("/avatars", "./data/avatars")
	return r
}
