// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/runtime"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/emote"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/feedback"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/mcpapi"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mood"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qq"
	"github.com/u-ai/backend/internal/realtime"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/temporal"
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
	r.POST("/api/shutdown", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "msg": "正在关闭服务..."})
		go func() {
			time.Sleep(300 * time.Millisecond)
			if triggerShutdown != nil {
				triggerShutdown()
			}
		}()
	})
	apiGroup := r.Group("/api")
	apiGroup.Use(middleware.AuthMiddleware())
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
		system.RegisterSystemRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.DataLifecycle, services.Memory, services.Profile, services.Episodic, services.Graph, services.Temporal)
		companion.RegisterCompanionRouter(apiGroup, services.Companion)
		qq.RegisterQQRouter(apiGroup, ctx)
		tts.RegisterTtsRouter(apiGroup, ctx)
		asr.RegisterAsrRouter(apiGroup, ctx)
		realtime.RegisterRealtimeRouter(apiGroup, ctx)
		vision.RegisterVisionRouter(apiGroup, ctx)
		embedding_config.RegisterEmbeddingConfigRouter(apiGroup, ctx)
		imagegen.RegisterImageGenRouter(apiGroup, ctx)
		desktoppet.RegisterDesktopPetRouter(apiGroup, ctx)
		processing.RegisterProcessingRouter(apiGroup, ctx)
		editing.RegisterEditingRouterWithService(apiGroup, services.EditingService)
		quality.RegisterQualityRouter(apiGroup, services.QualityService)
		installation.RegisterRoutes(apiGroup, services.InstallationService)
		installation.RegisterReleaseRoutes(apiGroup, services.ReleaseService)
		release.RegisterRoutes(apiGroup, services.NewReleaseService)
		behavior.RegisterRoutes(apiGroup, services.BehaviorService)
		system.RegisterPsycheAPIRouter(apiGroup)
		system.RegisterPsycheSnapshotRouter(apiGroup, ctx.DB)
		system.RegisterHealthRouter(apiGroup, services.CircuitBreakers, services.DataLifecycle, services.Reconciliation)
		ttsRepo := tts.NewRepository(ctx.DB)
		ttsSvc := tts.NewService(ttsRepo)
		system.RegisterVoiceEntryRouter(apiGroup, services.VoiceEntry, ttsSvc, services.DeliveryStore)
		safety.RegisterSafetyRouter(apiGroup, ctx.DB)
		delivery.RegisterSubmitRouter(apiGroup, services.DeliveryStore)
		extension.RegisterRouter(apiGroup, ctx, services.Extension)
		if services.KernelContainer != nil && services.KernelContainer.WASMRuntimeFactory != nil {
			wasmService := wasm_runtime.NewAPIService(services.KernelContainer.WASMRuntimeFactory, services.KernelContainer.WASMDefinitionRepo)
			wasmHandler := wasm_runtime.NewHTTPHandler(wasmService)
			wasmMux := http.NewServeMux()
			wasmHandler.Register(wasmMux)
			apiGroup.Any("/wasm/*wasmPath", gin.WrapH(wasmMux))
		}
		emote.RegisterRouter(apiGroup, services.Emote)
		mcpapi.RegisterRouter(apiGroup, ctx, mcpapi.Services{Repository: services.MCPRepository, Connections: services.MCPConnections, Auth: services.MCPAuth, Discovery: services.MCPDiscovery, Skills: services.MCPSkills, Secrets: services.MCPSecrets, Extensions: services.Extension, Features: services.MCPFeatures, Dependencies: services.MCPDependencies, Interactions: services.MCPInteractions})
		temporal.RegisterRouter(apiGroup, services.Temporal, services.RelTimeCoordinator)
		mood.RegisterMoodRouter(apiGroup, ctx)
	}
	runtime.RegisterInternalRoutes(r, services.DesktopPetRuntime)
	runtime.RegisterUserRoutes(apiGroup, services.DesktopPetRuntime)
	r.Static("/audio", "./data/tts_cache")
	r.Static("/exports", "./data/exports")
	r.Static("/voice", "./data/voice_msg")
	r.Static("/images", "./data/images")
	r.Static("/videos", "./data/videos")
	r.Static("/avatars", "./data/avatars")
	r.Static("/emote-assets", filepath.Join(config.AppCfg.Storage.DataDir, "emotes"))
	return r
}
