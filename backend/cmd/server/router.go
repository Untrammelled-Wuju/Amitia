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
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/feedback"
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
		profRepo := profile.NewRepository(ctx)
		profSvc := profile.NewService(profRepo, ctx)
		epiRepo := episodic.NewRepository(ctx)
		epiSvc := episodic.NewService(epiRepo, ctx)
		chatSvc := chat.NewService(chatRepo, ctx, memSvc, profSvc, epiSvc)
		chat.RegisterChatRouter(apiGroup, ctx, chatSvc)
		memory.RegisterMemoryRouter(apiGroup, ctx)

		profile.RegisterProfileRouter(apiGroup, ctx)
		proactive.RegisterProactiveRouter(apiGroup, ctx)
		episodic.RegisterEpisodicRouter(apiGroup, ctx)
		feedback.RegisterFeedbackRouter(apiGroup, ctx)
		agent.RegisterAgentRouter(apiGroup, ctx, profSvc, epiSvc)
		aicharacter.RegisterAICharacterRouter(apiGroup, ctx)
		system.RegisterSystemRouter(apiGroup, ctx, profSvc, epiSvc)
		companion.RegisterCompanionRouter(apiGroup, ctx)
		qq.RegisterQQRouter(apiGroup, ctx)
		tts.RegisterTtsRouter(apiGroup, ctx)
		asr.RegisterAsrRouter(apiGroup, ctx)
		realtime.RegisterRealtimeRouter(apiGroup, ctx)
		vision.RegisterVisionRouter(apiGroup, ctx)
	}

	r.Static("/audio", "./data/tts_cache")
	r.Static("/voice", "./data/voice_msg")
	r.Static("/images", "./data/images")
	r.Static("/videos", "./data/videos")
	return r
}
