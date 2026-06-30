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
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/feedback"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qq"
	"github.com/u-ai/backend/internal/realtime"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/internal/user"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/pkg/app"
)

func setupRouter(ctx *app.AppContext) *gin.Engine {
	if config.AppCfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(security.CorsMiddleware())

	apiGroup := r.Group("/api")
	{
		user.RegisterUserRouter(apiGroup, ctx)
		character.RegisterCharacterRouter(apiGroup, ctx)
		chatRepo := chat.NewRepository(ctx)
		memRepo := memory.NewRepository(ctx)
		memSvc := memory.NewService(memRepo, ctx)
		graphSvc := initGraphRouter()
		profRepo := profile.NewRepository(ctx)
		profSvc := profile.NewService(profRepo, ctx, graphSvc)
		epiRepo := episodic.NewRepository(ctx)
		epiSvc := episodic.NewService(epiRepo, ctx, graphSvc)
		wbRepo := worldbook.NewRepository(ctx)
		wbSvc := worldbook.NewService(wbRepo, ctx, graphSvc)
		visionRepo := vision.NewRepository(ctx.DB)
		visionSvc := vision.NewService(visionRepo)
		comp := chat.NewCompressor(ctx.DB)
		chatSvc := chat.NewService(chatRepo, ctx, memSvc, profSvc, epiSvc, wbSvc, comp, visionSvc)
		chat.RegisterChatRouter(apiGroup, ctx, chatSvc)
		memHandler := memory.RegisterMemoryRouter(apiGroup, ctx)
		apiGroup.GET("/memory/retrieval/stats", memHandler.RetrieveStats)
		apiGroup.GET("/memory/pipeline/status", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "data": chatSvc.GetPipelineStatus(), "msg": "操作成功"})
		})

		profile.RegisterProfileRouter(apiGroup, ctx)
		proactive.RegisterProactiveRouter(apiGroup, ctx)
		episodic.RegisterEpisodicRouter(apiGroup, ctx)
		worldbook.RegisterWorldBookRouter(apiGroup, ctx)
		feedback.RegisterFeedbackRouter(apiGroup, ctx)
		graph.RegisterGraphRouter(apiGroup, config.AppCfg.Surreal)
		agent.RegisterAgentRouter(apiGroup, ctx, profSvc, epiSvc)
		aicharacter.RegisterAICharacterRouter(apiGroup, ctx)
		system.RegisterSystemRouter(apiGroup, ctx, profSvc, epiSvc)
		companion.RegisterCompanionRouter(apiGroup, ctx)
		qq.RegisterQQRouter(apiGroup, ctx)
		tts.RegisterTtsRouter(apiGroup, ctx)
		asr.RegisterAsrRouter(apiGroup, ctx)
		realtime.RegisterRealtimeRouter(apiGroup, ctx)
		vision.RegisterVisionRouter(apiGroup, ctx)
		embedding_config.RegisterEmbeddingConfigRouter(apiGroup, ctx)
	}

	r.Static("/audio", "./data/tts_cache")
	r.Static("/voice", "./data/voice_msg")
	r.Static("/images", "./data/images")
	r.Static("/videos", "./data/videos")
	return r
}

func initGraphRouter() graph.Service {
	client, err := graph.NewClient(config.AppCfg.Surreal)
	if err != nil {
		return nil
	}
	return graph.NewService(client)
}
